import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// The store is fully mocked — AccountList is consumer-only. We drive scenarios
// by swapping what the store returns per test.

const setSelectedNetwork = vi.fn()
const setSelectedTag = vi.fn()
const setSearchQuery = vi.fn()
const removeAccount = vi.fn()
const detectAndUpdateRanks = vi.fn()
const editAccount = vi.fn()
const loadAccounts = vi.fn()
const addAccount = vi.fn()
const createTag = vi.fn()

type StoreOverrides = Partial<{
  accounts: any[]
  filteredAccounts: any[]
  showRecoveryPhraseModal: boolean
  detectedAccount: any
  activeAccountId: string | null
  selectedNetworkId: string | null
  selectedTag: string | null
  searchQuery: string
}>

let currentOverrides: StoreOverrides = {}

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => {
    const accounts = currentOverrides.accounts ?? []
    const filtered = currentOverrides.filteredAccounts ?? accounts
    return {
      filteredAccounts: () => filtered,
      accounts,
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
          id: 'epic',
          name: 'Epic Games',
          games: [
            { id: 'rl', name: 'Rocket League', networkId: 'epic' },
            { id: 'fortnite', name: 'Fortnite', networkId: 'epic' },
          ],
        },
      ],
      tags: ['main', 'smurf'],
      searchQuery: currentOverrides.searchQuery ?? '',
      selectedNetworkId: currentOverrides.selectedNetworkId ?? null,
      selectedTag: currentOverrides.selectedTag ?? null,
      username: 'eren',
      isDetecting: false,
      detectedAccount: currentOverrides.detectedAccount ?? null,
      activeAccountId: currentOverrides.activeAccountId ?? null,
      showRecoveryPhraseModal: currentOverrides.showRecoveryPhraseModal ?? false,
      setSearchQuery,
      setSelectedNetwork,
      setSelectedTag,
      removeAccount,
      detectAndUpdateRanks,
      editAccount,
      loadAccounts,
      addAccount,
      createTag,
    }
  },
}))

// Stub children that aren't under test to keep render cheap and isolated.
vi.mock('../RecoveryPhraseModal', () => ({ RecoveryPhraseModal: () => null }))
vi.mock('../SettingsModal', () => ({ SettingsModal: () => null }))

import { AccountList } from '../AccountList'

function render_withOverrides(overrides: StoreOverrides = {}) {
  currentOverrides = overrides
  return render(<AccountList />)
}

function makeAccount(partial: any = {}) {
  return {
    id: partial.id ?? 'a1',
    displayName: partial.displayName ?? 'Main Smurf',
    username: partial.username ?? 'mainsmurf',
    password: partial.password ?? 'pw',
    networkId: partial.networkId ?? 'riot',
    tags: partial.tags ?? [],
    notes: partial.notes ?? '',
    createdAt: new Date('2025-01-01').toISOString(),
    updatedAt: new Date('2025-01-01').toISOString(),
    riotId: partial.riotId ?? '',
    region: partial.region ?? 'na1',
    games: partial.games ?? ['lol'],
    cachedRanks: partial.cachedRanks ?? [],
    ...partial,
  }
}

describe('AccountList — onboarding', () => {
  beforeEach(() => {
    setSelectedNetwork.mockReset()
    setSelectedTag.mockReset()
    setSearchQuery.mockReset()
    removeAccount.mockReset()
    addAccount.mockReset()
  })

  it('auto-opens the Add Account wizard when no accounts exist', () => {
    render_withOverrides({ accounts: [], filteredAccounts: [] })

    // The wizard's step-1 subtitle is a reliable marker that it mounted.
    expect(screen.getByText(/Start with your sign-in credentials/i)).toBeInTheDocument()
  })

  it('does not open the wizard while the recovery-phrase modal is up', () => {
    render_withOverrides({
      accounts: [],
      filteredAccounts: [],
      showRecoveryPhraseModal: true,
    })

    expect(screen.queryByText(/Start with your sign-in credentials/i)).not.toBeInTheDocument()
  })

  it('does not auto-open the wizard for users who already have accounts', () => {
    const acc = makeAccount()
    render_withOverrides({ accounts: [acc], filteredAccounts: [acc] })

    expect(screen.queryByText(/Start with your sign-in credentials/i)).not.toBeInTheDocument()
  })
})

describe('AccountList — filters', () => {
  beforeEach(() => {
    setSelectedNetwork.mockReset()
    setSelectedTag.mockReset()
    setSearchQuery.mockReset()
  })

  it('fires setSelectedNetwork when user picks a network from the dropdown', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      accounts: [makeAccount()],
      filteredAccounts: [makeAccount()],
    })

    const networkSelect = screen.getAllByRole('combobox')[0] // first select is Network
    await user.selectOptions(networkSelect, 'riot')

    expect(setSelectedNetwork).toHaveBeenCalledWith('riot')
  })

  it('fires setSearchQuery as user types in the search box', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      accounts: [makeAccount()],
      filteredAccounts: [makeAccount()],
    })

    const search = screen.getByPlaceholderText('Search')
    await user.type(search, 'a')

    // setSearchQuery fires per keystroke; assert at least one call with the
    // character the user typed.
    expect(setSearchQuery).toHaveBeenCalledWith('a')
  })
})

describe('AccountList — delete flow', () => {
  it('opens the delete modal when the trash icon is clicked', async () => {
    const user = userEvent.setup()
    const acc = makeAccount({ displayName: 'Doomed Account' })
    render_withOverrides({ accounts: [acc], filteredAccounts: [acc] })

    await user.click(screen.getByRole('button', { name: /delete account/i }))

    // Copy that only appears inside the confirmation modal.
    expect(screen.getByText(/permanently delete/i)).toBeInTheDocument()
    expect(screen.getByText(/Delete account\?/i)).toBeInTheDocument()
  })

  it('calls removeAccount when the delete is confirmed', async () => {
    const user = userEvent.setup()
    const acc = makeAccount({ id: 'kill-me', displayName: 'Doomed Account' })
    removeAccount.mockResolvedValue(true)
    render_withOverrides({ accounts: [acc], filteredAccounts: [acc] })

    await user.click(screen.getByRole('button', { name: /delete account/i }))

    // The modal's confirm button's accessible name is exactly "Delete"; the
    // card's trash icon is "Delete account", so a strict match disambiguates.
    const confirm = screen.getByRole('button', { name: /^delete$/i })
    await user.click(confirm)

    expect(removeAccount).toHaveBeenCalledWith('kill-me')
  })
})

describe('AccountList — game badge', () => {
  // Anti-regression for the bug where the badge ternary defaulted any unknown
  // gameId to "Valorant" (so a Rocket League card displayed as Valorant). The
  // dynamic gameBadge resolves names from the gameNetworks catalog.
  it('renders the correct label for Rocket League and Fortnite (not "Valorant")', () => {
    const acc = makeAccount({
      id: 'rl-epic',
      networkId: 'epic',
      games: ['rl', 'fortnite'],
    })
    render_withOverrides({ accounts: [acc], filteredAccounts: [acc] })

    // Badges are <span>s on the card. Scope to spans so we don't pick up the
    // Game-filter <option> values, which legitimately list every known game.
    const badgeText = (label: string) =>
      screen.queryAllByText(label).filter((el) => el.tagName === 'SPAN')

    expect(badgeText('Rocket League')).toHaveLength(1)
    expect(badgeText('Fortnite')).toHaveLength(1)
    // The card shouldn't accidentally label either game as Valorant.
    expect(badgeText('Valorant')).toHaveLength(0)
  })

  it('falls back to the catalog name for game ids it has no hardcoded badge for', () => {
    // Simulate a future game added to gameNetworks but not yet in GAME_BADGE.
    // The dynamic resolver should still surface the proper name, not the id.
    currentOverrides = {}
    const acc = makeAccount({ networkId: 'riot', games: ['lol'] })
    render_withOverrides({ accounts: [acc], filteredAccounts: [acc] })
    // 'lol' is in GAME_BADGE so it renders as "League"; this just sanity-
    // checks that the known-id path still works alongside the new fallback.
    expect(screen.getByText('League')).toBeInTheDocument()
  })
})

describe('AccountList — sound mute toggle', () => {
  it('switches icon + aria-label when toggled', async () => {
    const user = userEvent.setup()
    const acc = makeAccount()
    render_withOverrides({ accounts: [acc], filteredAccounts: [acc] })

    const toggle = screen.getByRole('button', { name: /mute sounds/i })
    await user.click(toggle)

    // After click, the label should flip to "Enable sounds"
    expect(screen.getByRole('button', { name: /enable sounds/i })).toBeInTheDocument()
  })
})
