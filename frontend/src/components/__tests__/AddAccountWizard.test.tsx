import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const addAccount = vi.fn()
const createTag = vi.fn()

// Mock catalog mirrors the production shape: Riot owns three games (single-store
// each), and Rocket League is the cross-store case shipping under both Epic
// and Steam — that's the multi-network branch we want to exercise.
vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    gameNetworks: [
      {
        id: 'riot',
        name: 'Riot Games',
        // SharedAccount=true encodes that one Riot login spans LoL + TFT + Valorant.
        sharedAccount: true,
        games: [
          { id: 'lol', name: 'League of Legends', networkId: 'riot' },
          { id: 'tft', name: 'Teamfight Tactics', networkId: 'riot' },
          { id: 'valorant', name: 'Valorant', networkId: 'riot' },
        ],
      },
      {
        id: 'epic',
        name: 'Epic Games',
        // One Epic login covers Rocket League AND Fortnite — picking either
        // auto-tags the account with both, mirroring the Riot LoL/TFT/Valorant
        // pattern.
        sharedAccount: true,
        games: [
          { id: 'rl', name: 'Rocket League', networkId: 'epic' },
          { id: 'fortnite', name: 'Fortnite', networkId: 'epic' },
        ],
      },
      {
        id: 'steam',
        name: 'Steam',
        sharedAccount: false,
        games: [{ id: 'rl', name: 'Rocket League', networkId: 'steam' }],
      },
    ],
    tags: ['main', 'smurf'],
    addAccount,
    createTag,
  }),
}))

import { AddAccountWizard, fuzzyMatch } from '../AddAccountWizard'

// Helper: drive the identity step (always first).
async function fillIdentity(user: ReturnType<typeof userEvent.setup>, username = 'smurf1', password = 'pw') {
  await user.type(screen.getByPlaceholderText('Enter username'), username)
  await user.type(screen.getByPlaceholderText('Enter password'), password)
  await user.click(screen.getByRole('button', { name: /next/i }))
}

describe('fuzzyMatch', () => {
  it('matches subsequence regardless of gaps', () => {
    expect(fuzzyMatch('lol', 'League of Legends')).toBe(true)
    expect(fuzzyMatch('riot', 'Riot Games')).toBe(true)
    expect(fuzzyMatch('tft', 'Teamfight Tactics')).toBe(true)
  })

  it('rejects when characters are out of order', () => {
    expect(fuzzyMatch('xyz', 'League of Legends')).toBe(false)
  })

  it('treats empty query as match', () => {
    expect(fuzzyMatch('', 'anything')).toBe(true)
    expect(fuzzyMatch('  ', 'anything')).toBe(true)
  })
})

describe('AddAccountWizard', () => {
  beforeEach(() => {
    addAccount.mockReset()
    createTag.mockReset()
  })

  it('masks the password field with type="password"', () => {
    render(<AddAccountWizard onClose={vi.fn()} />)
    const pw = screen.getByPlaceholderText('Enter password') as HTMLInputElement
    expect(pw.type).toBe('password')
  })

  it('requires username and password before advancing from the identity step', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    const next = screen.getByRole('button', { name: /next/i })
    expect(next).toBeDisabled()

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    expect(next).toBeDisabled()

    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    expect(next).toBeEnabled()
  })

  it('shows games (not networks) on the second step — game-first flow', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)
    await fillIdentity(user)

    expect(screen.getByRole('button', { name: /league of legends/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /rocket league/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument()
  })

  it('requires a game pick before advancing', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)
    await fillIdentity(user)

    const next = screen.getByRole('button', { name: /next/i })
    expect(next).toBeDisabled()

    await user.click(screen.getByRole('button', { name: /league of legends/i }))
    expect(next).toBeEnabled()
  })

  it('filters game tiles with fuzzy search', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)
    await fillIdentity(user)

    expect(screen.getByRole('button', { name: /league of legends/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /rocket league/i })).toBeInTheDocument()

    await user.type(screen.getByLabelText(/search games/i), 'rocket')

    expect(screen.queryByRole('button', { name: /league of legends/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /rocket league/i })).toBeInTheDocument()
  })

  it('skips the network step for single-store games (League of Legends → Riot only)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /league of legends/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Should land directly on the details step (Tags label is unique to it).
    expect(screen.getByText(/^Tags$/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /add account/i })).toBeInTheDocument()
  })

  it('inserts the network step for cross-store games (Rocket League → Epic + Steam)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Network step shows both stores as tiles.
    expect(screen.getByRole('button', { name: /epic games/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /steam/i })).toBeInTheDocument()
  })

  it('does not pre-pick a store on the network step (forces explicit choice)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Both stores rendered, neither pre-selected — different credentials per
    // store mean we shouldn't guess.
    expect(screen.getByRole('button', { name: /epic games/i })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: /steam/i })).toHaveAttribute('aria-pressed', 'false')

    // Next is blocked until one is picked.
    expect(screen.getByRole('button', { name: /^next$/i })).toBeDisabled()
  })

  it('network step is single-select — picking a second store swaps the choice', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    await user.click(screen.getByRole('button', { name: /epic games/i }))
    expect(screen.getByRole('button', { name: /epic games/i })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /steam/i })).toHaveAttribute('aria-pressed', 'false')

    // Switching to Steam swaps the selection — no multi-select.
    await user.click(screen.getByRole('button', { name: /steam/i }))
    expect(screen.getByRole('button', { name: /epic games/i })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: /steam/i })).toHaveAttribute('aria-pressed', 'true')
  })

  it('submits exactly one account record for a multi-store game (Steam pick)', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<AddAccountWizard onClose={onClose} />)

    await fillIdentity(user, 'rl-steam', 'hunter2')
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    await user.click(screen.getByRole('button', { name: /steam/i }))
    await user.click(screen.getByRole('button', { name: /^next$/i }))

    // Submit label is always plain "Add Account" — no batch creation.
    await user.click(screen.getByRole('button', { name: /add account/i }))

    expect(addAccount).toHaveBeenCalledTimes(1)
    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('steam')
    expect(payload.username).toBe('rl-steam')
    expect(payload.games).toEqual(['rl'])
    expect(payload.riotId).toBe('')
    expect(payload.region).toBe('')
    expect(onClose).toHaveBeenCalled()
  })

  it('auto-links Rocket League when picking Fortnite (Epic single-network → skip network step → still links)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user, 'fortnite-main', 'pw')
    // Fortnite is Epic-only, so the network step is skipped — we land
    // directly on details. The auto-link must fire even without an explicit
    // network pick because Epic was implicitly chosen as the only option.
    await user.click(screen.getByRole('button', { name: /fortnite/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    const note = screen.getByRole('note', { name: /linked games/i })
    expect(note).toHaveTextContent(/fortnite/i)
    expect(note).toHaveTextContent(/rocket league/i)

    await user.click(screen.getByRole('button', { name: /add account/i }))

    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('epic')
    expect([...payload.games].sort()).toEqual(['fortnite', 'rl'])
  })

  it('switching games on the game step clears stale auto-link state (Fortnite → swap to RL → Steam)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user, 'tester', 'pw')

    // Pick Fortnite first — it's Epic-only so selectedNetwork pre-fills to epic.
    // Then swap to Rocket League on the same step (which is multi-store and
    // resets selectedNetwork). Pick Steam → Add. Steam isn't shared so games
    // must contain only 'rl' — no Fortnite leaking from the earlier pick.
    await user.click(screen.getByRole('button', { name: /fortnite/i }))
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.click(screen.getByRole('button', { name: /steam/i }))
    await user.click(screen.getByRole('button', { name: /^next$/i }))
    await user.click(screen.getByRole('button', { name: /add account/i }))

    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('steam')
    expect(payload.games).toEqual(['rl'])
  })

  it('auto-links Fortnite when picking Rocket League on Epic (one Epic login covers both)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user, 'rl-epic', 'pw')
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    await user.click(screen.getByRole('button', { name: /epic games/i }))
    await user.click(screen.getByRole('button', { name: /^next$/i }))

    // Linked-games note should mention both titles since Epic.sharedAccount=true.
    const note = screen.getByRole('note', { name: /linked games/i })
    expect(note).toHaveTextContent(/rocket league/i)
    expect(note).toHaveTextContent(/fortnite/i)

    await user.click(screen.getByRole('button', { name: /add account/i }))

    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('epic')
    expect([...payload.games].sort()).toEqual(['fortnite', 'rl'])
  })

  it('shows Riot ID + Region only when the chosen game lives on Riot', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    // Rocket League → Epic + Steam: no Riot fields on details.
    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /rocket league/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.click(screen.getByRole('button', { name: /^next$/i }))

    expect(screen.queryByPlaceholderText(/GameName#TAG/i)).not.toBeInTheDocument()
  })

  it('shows Riot ID + Region for League of Legends (Riot game)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /league of legends/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    expect(screen.getByPlaceholderText(/GameName#TAG/i)).toBeInTheDocument()
  })

  it('auto-links Riot sibling games (one login covers LoL + TFT + Valorant)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user, 'riot-main', 'pw')
    await user.click(screen.getByRole('button', { name: /league of legends/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Shared-account networks render an info note instead of opt-in tiles —
    // the user can't (and shouldn't be able to) un-link siblings on the same
    // login. The note must mention the sibling games so it's discoverable.
    const note = screen.getByRole('note', { name: /linked games/i })
    expect(note).toHaveTextContent(/league of legends/i)
    expect(note).toHaveTextContent(/teamfight tactics/i)
    expect(note).toHaveTextContent(/valorant/i)

    // No interactive tiles for TFT/Valorant on this step (the only buttons
    // labeled with their names live on the game-step which is no longer rendered).
    expect(screen.queryByRole('button', { name: /teamfight tactics/i })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /add account/i }))

    expect(addAccount).toHaveBeenCalledTimes(1)
    const games: string[] = addAccount.mock.calls[0][0].games
    // Order isn't load-bearing — assert membership.
    expect(games.sort()).toEqual(['lol', 'tft', 'valorant'])
  })

  it('lets the user toggle existing tags and create new ones', async () => {
    const user = userEvent.setup()
    createTag.mockResolvedValue(undefined)
    const onClose = vi.fn()
    render(<AddAccountWizard onClose={onClose} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /league of legends/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    await user.click(screen.getByRole('button', { name: /^main$/i }))

    const tagInput = screen.getByLabelText(/create a new tag/i)
    await user.type(tagInput, 'ranked-grind{Enter}')

    expect(createTag).toHaveBeenCalledWith('ranked-grind')
    expect(tagInput).toHaveValue('')

    await user.click(screen.getByRole('button', { name: /add account/i }))

    expect(addAccount).toHaveBeenCalledTimes(1)
    expect(addAccount.mock.calls[0][0].tags).toEqual(['main', 'ranked-grind'])
  })

  it('Back button returns to previous step without losing data', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /back/i }))

    expect(screen.getByPlaceholderText('Enter username')).toHaveValue('smurf1')
    expect(screen.getByPlaceholderText('Enter password')).toHaveValue('pw')
  })

  it('shows a Custom tile on the game step as an escape hatch', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)
    await fillIdentity(user)

    expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument()
  })

  it('still shows Custom when no games match the search query', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)
    await fillIdentity(user)

    await user.type(screen.getByLabelText(/search games/i), 'zzznotagame')

    expect(screen.queryByRole('button', { name: /league of legends/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /rocket league/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument()
  })

  it('requires a network name before submitting a Custom account', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)
    await user.click(screen.getByRole('button', { name: /custom/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    const submit = screen.getByRole('button', { name: /add account/i })
    expect(submit).toBeDisabled()

    await user.type(screen.getByPlaceholderText(/steam, epic games/i), 'Battle.net')
    expect(submit).toBeEnabled()
  })

  it('submits a Custom account with customNetwork + customGame and no riot/games fields', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<AddAccountWizard onClose={onClose} />)

    await fillIdentity(user, 'cs-main', 'hunter2')
    await user.click(screen.getByRole('button', { name: /custom/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    await user.type(screen.getByPlaceholderText(/steam, epic games/i), 'Battle.net')
    await user.type(screen.getByPlaceholderText(/cs2, apex legends/i), 'Overwatch 2')
    await user.click(screen.getByRole('button', { name: /add account/i }))

    expect(addAccount).toHaveBeenCalledTimes(1)
    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('custom')
    expect(payload.customNetwork).toBe('Battle.net')
    expect(payload.customGame).toBe('Overwatch 2')
    expect(payload.games).toEqual([])
    expect(payload.riotId).toBe('')
    expect(payload.region).toBe('')
    expect(onClose).toHaveBeenCalled()
  })

  it('clears custom fields when switching back from Custom to a real game', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await fillIdentity(user)

    await user.click(screen.getByRole('button', { name: /custom/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.type(screen.getByPlaceholderText(/steam, epic games/i), 'Battle.net')
    await user.click(screen.getByRole('button', { name: /back/i }))
    await user.click(screen.getByRole('button', { name: /league of legends/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.click(screen.getByRole('button', { name: /add account/i }))

    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('riot')
    expect(payload.customNetwork).toBe('')
    expect(payload.customGame).toBe('')
    // Riot SharedAccount=true means the LoL pick auto-tags TFT + Valorant too.
    expect([...payload.games].sort()).toEqual(['lol', 'tft', 'valorant'])
  })

  describe('sticky mode (first-account onboarding)', () => {
    it('ignores backdrop clicks — only Cancel exits', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      const { container } = render(<AddAccountWizard onClose={onClose} sticky />)

      // The backdrop is the outermost fixed-inset wrapper rendered by Modal.
      const backdrop = container.querySelector('.fixed.inset-0') as HTMLElement
      expect(backdrop).toBeTruthy()
      await user.click(backdrop)

      expect(onClose).not.toHaveBeenCalled()
    })

    it('ignores the Escape key', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      render(<AddAccountWizard onClose={onClose} sticky />)

      await user.keyboard('{Escape}')

      expect(onClose).not.toHaveBeenCalled()
    })

    it('still closes when the user clicks Cancel', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      render(<AddAccountWizard onClose={onClose} sticky />)

      await user.click(screen.getByRole('button', { name: /cancel/i }))
      expect(onClose).toHaveBeenCalled()
    })

    it('non-sticky mode (default) still allows backdrop dismissal', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      const { container } = render(<AddAccountWizard onClose={onClose} />)

      const backdrop = container.querySelector('.fixed.inset-0') as HTMLElement
      await user.click(backdrop)

      expect(onClose).toHaveBeenCalled()
    })
  })
})
