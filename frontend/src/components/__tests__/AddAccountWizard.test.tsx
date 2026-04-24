import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const addAccount = vi.fn()
const createTag = vi.fn()

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    gameNetworks: [
      {
        id: 'riot',
        name: 'Riot Games',
        games: [
          { id: 'lol', name: 'League of Legends', networkId: 'riot' },
          { id: 'tft', name: 'Teamfight Tactics', networkId: 'riot' },
          { id: 'valorant', name: 'Valorant', networkId: 'riot' },
        ],
      },
      {
        id: 'steam',
        name: 'Steam',
        games: [{ id: 'cs2', name: 'Counter-Strike 2', networkId: 'steam' }],
      },
    ],
    tags: ['main', 'smurf'],
    addAccount,
    createTag,
  }),
}))

import { AddAccountWizard, fuzzyMatch } from '../AddAccountWizard'

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

  it('requires username and password before advancing from step 1', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    const next = screen.getByRole('button', { name: /next/i })
    expect(next).toBeDisabled()

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    expect(next).toBeDisabled()

    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    expect(next).toBeEnabled()
  })

  it('requires network selection before advancing from step 2', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 2 — network step. Next should be disabled until a tile is picked.
    const nextOnStep2 = screen.getByRole('button', { name: /next/i })
    expect(nextOnStep2).toBeDisabled()

    await user.click(screen.getByRole('button', { name: /riot games/i }))
    expect(nextOnStep2).toBeEnabled()
  })

  it('filters network tiles with fuzzy search', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Both networks visible initially
    expect(screen.getByRole('button', { name: /riot games/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /steam/i })).toBeInTheDocument()

    await user.type(screen.getByLabelText(/search networks/i), 'ste')

    expect(screen.queryByRole('button', { name: /riot games/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /steam/i })).toBeInTheDocument()
  })

  it('submits with sensible defaults when user skips step 3', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<AddAccountWizard onClose={onClose} />)

    // Step 1
    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw123')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 2
    await user.click(screen.getByRole('button', { name: /riot games/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 3 — skip straight to Add Account
    await user.click(screen.getByRole('button', { name: /add account/i }))

    expect(addAccount).toHaveBeenCalledTimes(1)
    const payload = addAccount.mock.calls[0][0]
    expect(payload.username).toBe('smurf1')
    expect(payload.password).toBe('pw123')
    expect(payload.networkId).toBe('riot')
    expect(payload.displayName).toBe('smurf1') // defaults to username
    expect(payload.games).toEqual(['lol', 'tft', 'valorant']) // defaults to all network games
    expect(payload.region).toBe('na1')
    expect(onClose).toHaveBeenCalled()
  })

  it('pre-selects all games when a network is picked (opt-out UX)', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.click(screen.getByRole('button', { name: /riot games/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // On step 3 every Riot game tile should already be selected.
    expect(screen.getByRole('button', { name: /league of legends/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(screen.getByRole('button', { name: /teamfight tactics/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(screen.getByRole('button', { name: /valorant/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
  })

  it('lets the user toggle existing tags and create new ones', async () => {
    const user = userEvent.setup()
    createTag.mockResolvedValue(undefined)
    const onClose = vi.fn()
    render(<AddAccountWizard onClose={onClose} />)

    // Advance to step 3
    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.click(screen.getByRole('button', { name: /riot games/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Toggle existing tag
    await user.click(screen.getByRole('button', { name: /main/i }))

    // Create a brand new tag via Enter key
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

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    await user.click(screen.getByRole('button', { name: /back/i }))

    expect(screen.getByPlaceholderText('Enter username')).toHaveValue('smurf1')
    expect(screen.getByPlaceholderText('Enter password')).toHaveValue('pw')
  })

  it('shows a Custom tile on the network step as an escape hatch', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument()
  })

  it('still shows Custom when no networks match the query', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Type a nonsense query that matches no real network
    await user.type(screen.getByLabelText(/search networks/i), 'zzzzfortnite')

    expect(screen.queryByRole('button', { name: /riot games/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /steam/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument()
  })

  it('requires a network name before submitting a Custom account', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    // Step 1
    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 2 — pick Custom
    await user.click(screen.getByRole('button', { name: /custom/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 3 — Add Account should be disabled until network name is entered
    const submit = screen.getByRole('button', { name: /add account/i })
    expect(submit).toBeDisabled()

    await user.type(screen.getByPlaceholderText(/steam, epic games/i), 'Steam')
    expect(submit).toBeEnabled()
  })

  it('submits a Custom account with customNetwork + customGame and no riot/games fields', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<AddAccountWizard onClose={onClose} />)

    // Step 1
    await user.type(screen.getByPlaceholderText('Enter username'), 'cs-main')
    await user.type(screen.getByPlaceholderText('Enter password'), 'hunter2')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 2 — Custom
    await user.click(screen.getByRole('button', { name: /custom/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Step 3 — custom inputs
    await user.type(screen.getByPlaceholderText(/steam, epic games/i), 'Steam')
    await user.type(screen.getByPlaceholderText(/cs2, apex legends/i), 'Counter-Strike 2')
    await user.click(screen.getByRole('button', { name: /add account/i }))

    expect(addAccount).toHaveBeenCalledTimes(1)
    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('custom')
    expect(payload.customNetwork).toBe('Steam')
    expect(payload.customGame).toBe('Counter-Strike 2')
    expect(payload.games).toEqual([])
    expect(payload.riotId).toBe('')
    expect(payload.region).toBe('')
    expect(onClose).toHaveBeenCalled()
  })

  it('clears custom fields when switching back from Custom to a real network', async () => {
    const user = userEvent.setup()
    render(<AddAccountWizard onClose={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'smurf1')
    await user.type(screen.getByPlaceholderText('Enter password'), 'pw')
    await user.click(screen.getByRole('button', { name: /next/i }))

    // Pick Custom, fill the name, then switch to Riot
    await user.click(screen.getByRole('button', { name: /custom/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.type(screen.getByPlaceholderText(/steam, epic games/i), 'Steam')
    await user.click(screen.getByRole('button', { name: /back/i }))
    await user.click(screen.getByRole('button', { name: /riot games/i }))
    await user.click(screen.getByRole('button', { name: /next/i }))
    await user.click(screen.getByRole('button', { name: /add account/i }))

    const payload = addAccount.mock.calls[0][0]
    expect(payload.networkId).toBe('riot')
    expect(payload.customNetwork).toBe('')
    expect(payload.customGame).toBe('')
    expect(payload.games).toEqual(['lol', 'tft', 'valorant'])
  })
})
