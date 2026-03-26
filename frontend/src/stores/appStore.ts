import { create } from 'zustand'
import type { AppState, AccountInput } from '../lib/types'
import { models } from '../../wailsjs/go/models'
import {
  VaultExists,
  IsUnlocked,
  CreateVault,
  Unlock,
  Lock,
  GetAllAccounts,
  GetGameNetworks,
  GetAllTags,
  GetSettings,
  CreateAccount,
  UpdateAccount,
  DeleteAccount,
  AddTag,
  GetUsername,
  GetStoredUsername,
  DetectSignedInAccount,
  MatchAndUpdateAccount,
  IsRiotClientRunning,
  SetWindowSizeLogin,
  SetWindowSizeMain,
} from '../../wailsjs/go/main/App'
import { riotclient } from '../../wailsjs/go/models'

interface AppStore {
  // App state
  appState: AppState
  error: string | null

  // User info
  username: string
  storedUsername: string // Pre-filled from vault for login

  // Data
  accounts: models.Account[]
  gameNetworks: models.GameNetwork[]
  tags: string[]
  settings: models.Settings | null

  // Detection state
  detectedAccount: riotclient.DetectedAccount | null
  isDetecting: boolean
  riotClientRunning: boolean
  activeAccountId: string | null // Currently signed-in account ID
  isPolling: boolean

  // Filters
  searchQuery: string
  selectedNetworkId: string | null
  selectedTag: string | null

  // Selected account for editing
  selectedAccountId: string | null

  // Actions
  initialize: () => Promise<void>
  createVault: (username: string, password: string) => Promise<boolean>
  unlock: (username: string, password: string) => Promise<boolean>
  lock: () => void
  clearError: () => void

  // Account actions
  loadAccounts: () => Promise<void>
  addAccount: (account: AccountInput) => Promise<boolean>
  editAccount: (account: AccountInput & { id: string }) => Promise<boolean>
  removeAccount: (id: string) => Promise<boolean>
  selectAccount: (id: string | null) => void

  // Tag actions
  createTag: (tag: string) => Promise<void>

  // Filter actions
  setSearchQuery: (query: string) => void
  setSelectedNetwork: (networkId: string | null) => void
  setSelectedTag: (tag: string | null) => void

  // Detection actions
  detectAndUpdateRanks: () => Promise<string | null>
  checkRiotClient: () => Promise<boolean>
  pollForActiveAccount: () => Promise<void>
  startPolling: () => void
  stopPolling: () => void

  // Computed
  filteredAccounts: () => models.Account[]
}

export const useAppStore = create<AppStore>((set, get) => ({
  // Initial state
  appState: 'loading',
  error: null,
  username: '',
  storedUsername: '',
  accounts: [],
  gameNetworks: [],
  tags: [],
  settings: null,
  detectedAccount: null,
  isDetecting: false,
  riotClientRunning: false,
  activeAccountId: null,
  isPolling: false,
  searchQuery: '',
  selectedNetworkId: null,
  selectedTag: null,
  selectedAccountId: null,

  // Initialize app - check vault state
  initialize: async () => {
    try {
      const exists = await VaultExists()
      const unlocked = await IsUnlocked()

      if (unlocked) {
        // Load data
        const [accounts, networks, tags, settings, username] = await Promise.all([
          GetAllAccounts(),
          GetGameNetworks(),
          GetAllTags(),
          GetSettings(),
          GetUsername(),
        ])
        set({
          appState: 'unlocked',
          username,
          accounts: accounts || [],
          gameNetworks: networks || [],
          tags: tags || [],
          settings,
        })
        // Resize to main window size
        SetWindowSizeMain()
        // Start polling for active account
        get().startPolling()
      } else if (exists) {
        // Get stored username for pre-filling login form
        try {
          const storedUsername = await GetStoredUsername()
          set({ appState: 'locked', storedUsername })
        } catch {
          set({ appState: 'locked' })
        }
      } else {
        set({ appState: 'create' })
      }
    } catch (e) {
      set({ error: String(e) })
    }
  },

  // Create new vault
  createVault: async (username: string, password: string) => {
    try {
      await CreateVault(username, password)
      const [accounts, networks, tags, settings] = await Promise.all([
        GetAllAccounts(),
        GetGameNetworks(),
        GetAllTags(),
        GetSettings(),
      ])
      set({
        appState: 'unlocked',
        username,
        accounts: accounts || [],
        gameNetworks: networks || [],
        tags: tags || [],
        settings,
        error: null,
      })
      // Resize to main window size
      SetWindowSizeMain()
      // Start polling for active account
      get().startPolling()
      return true
    } catch (e) {
      set({ error: String(e) })
      return false
    }
  },

  // Unlock vault
  unlock: async (username: string, password: string) => {
    try {
      await Unlock(username, password)
      const [accounts, networks, tags, settings] = await Promise.all([
        GetAllAccounts(),
        GetGameNetworks(),
        GetAllTags(),
        GetSettings(),
      ])
      set({
        appState: 'unlocked',
        username,
        accounts: accounts || [],
        gameNetworks: networks || [],
        tags: tags || [],
        settings,
        error: null,
      })
      // Resize to main window size
      SetWindowSizeMain()
      // Start polling for active account
      get().startPolling()
      return true
    } catch (e) {
      set({ error: 'Invalid username or password' })
      return false
    }
  },

  // Lock vault
  lock: () => {
    const currentUsername = get().username
    // Stop polling
    get().stopPolling()
    Lock()
    set({
      appState: 'locked',
      storedUsername: currentUsername, // Pre-fill for next login
      username: '',
      accounts: [],
      tags: [],
      settings: null,
      selectedAccountId: null,
      activeAccountId: null,
      detectedAccount: null,
    })
    // Resize back to login window size
    SetWindowSizeLogin()
  },

  clearError: () => set({ error: null }),

  // Load accounts
  loadAccounts: async () => {
    try {
      const accounts = await GetAllAccounts()
      set({ accounts: accounts || [] })
    } catch (e) {
      set({ error: String(e) })
    }
  },

  // Add account
  addAccount: async (account) => {
    try {
      const created = await CreateAccount(account as any)
      if (created) {
        await get().loadAccounts()
        return true
      }
      return false
    } catch (e) {
      set({ error: String(e) })
      return false
    }
  },

  // Edit account
  editAccount: async (account) => {
    try {
      const updated = await UpdateAccount(account as any)
      if (updated) {
        await get().loadAccounts()
        return true
      }
      return false
    } catch (e) {
      set({ error: String(e) })
      return false
    }
  },

  // Remove account
  removeAccount: async (id) => {
    try {
      await DeleteAccount(id)
      await get().loadAccounts()
      set({ selectedAccountId: null })
      return true
    } catch (e) {
      set({ error: String(e) })
      return false
    }
  },

  selectAccount: (id) => set({ selectedAccountId: id }),

  // Create tag
  createTag: async (tag) => {
    try {
      await AddTag(tag)
      const tags = await GetAllTags()
      set({ tags: tags || [] })
    } catch (e) {
      set({ error: String(e) })
    }
  },

  // Filters
  setSearchQuery: (query) => set({ searchQuery: query }),
  setSelectedNetwork: (networkId) => set({ selectedNetworkId: networkId }),
  setSelectedTag: (tag) => set({ selectedTag: tag }),

  // Detection actions
  checkRiotClient: async () => {
    try {
      const running = await IsRiotClientRunning()
      set({ riotClientRunning: running })
      return running
    } catch {
      set({ riotClientRunning: false })
      return false
    }
  },

  detectAndUpdateRanks: async () => {
    set({ isDetecting: true, error: null })
    try {
      const detected = await DetectSignedInAccount()
      if (!detected) {
        set({ isDetecting: false, detectedAccount: null, activeAccountId: null })
        return null
      }
      set({ detectedAccount: detected })

      // Try to match and update existing account
      const matchedId = await MatchAndUpdateAccount(detected)

      // Reload accounts to get updated data
      await get().loadAccounts()

      set({ isDetecting: false, activeAccountId: matchedId || null })
      return matchedId || null
    } catch (e) {
      set({ isDetecting: false, error: String(e), activeAccountId: null })
      return null
    }
  },

  // Lightweight polling - just checks who's signed in without updating ranks
  pollForActiveAccount: async () => {
    try {
      const detected = await DetectSignedInAccount()

      // Check if we got a valid detection (has RiotID)
      if (!detected || !detected.RiotID) {
        set({ detectedAccount: null, activeAccountId: null, riotClientRunning: false })
        return
      }

      set({ detectedAccount: detected, riotClientRunning: true })

      // Match to existing account without updating ranks (lightweight)
      const { accounts } = get()
      const riotIdLower = detected.RiotID.toLowerCase()
      const matched = accounts.find(acc =>
        acc.riotId?.toLowerCase() === riotIdLower ||
        acc.puuid === detected.PUUID
      )

      set({ activeAccountId: matched?.id || null })
    } catch {
      // Client closed or error - clear the active state
      set({ detectedAccount: null, riotClientRunning: false, activeAccountId: null })
    }
  },

  startPolling: () => {
    if (get().isPolling) return
    set({ isPolling: true })

    // Initial poll
    get().pollForActiveAccount()

    // Poll every 10 seconds
    const intervalId = setInterval(() => {
      if (get().isPolling) {
        get().pollForActiveAccount()
      }
    }, 10000)

    // Store interval ID for cleanup (using window)
    ;(window as any).__pollingIntervalId = intervalId
  },

  stopPolling: () => {
    set({ isPolling: false })
    const intervalId = (window as any).__pollingIntervalId
    if (intervalId) {
      clearInterval(intervalId)
      ;(window as any).__pollingIntervalId = null
    }
  },

  // Filtered accounts
  filteredAccounts: () => {
    const { accounts, searchQuery, selectedNetworkId, selectedTag } = get()

    return accounts.filter(acc => {
      // Network filter
      if (selectedNetworkId && acc.networkId !== selectedNetworkId) {
        return false
      }

      // Tag filter
      if (selectedTag && !acc.tags.includes(selectedTag)) {
        return false
      }

      // Search filter
      if (searchQuery) {
        const query = searchQuery.toLowerCase()
        return (
          acc.displayName.toLowerCase().includes(query) ||
          acc.username.toLowerCase().includes(query) ||
          acc.notes.toLowerCase().includes(query)
        )
      }

      return true
    })
  },
}))
