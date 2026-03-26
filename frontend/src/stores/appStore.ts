import { create } from 'zustand'
import type { AppState, AccountInput } from '../lib/types'
import { models } from '../../wailsjs/go/models'
import {
  VaultExists,
  IsUnlocked,
  CreateVault,
  CreateVaultWithHint,
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
  GetPasswordHint,
  ChangePassword,
  UpdatePasswordHint,
  UpdateSettings,
  DetectSignedInAccount,
  MatchAndUpdateAccount,
  IsRiotClientRunning,
  SetWindowSizeLogin,
  SetWindowSizeMain,
  GetAppVersion,
  CheckForUpdates,
  DownloadAndInstallUpdate,
  OpenReleasePage,
} from '../../wailsjs/go/main/App'
import { riotclient, updater } from '../../wailsjs/go/models'

interface AppStore {
  // App state
  appState: AppState
  error: string | null

  // User info
  username: string
  storedUsername: string // Pre-filled from vault for login
  passwordHint: string // Password hint displayed on lock screen

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

  // Update state
  appVersion: string
  updateInfo: updater.UpdateInfo | null
  isCheckingForUpdates: boolean
  showUpdateModal: boolean

  // Actions
  initialize: () => Promise<void>
  createVault: (username: string, password: string, hint?: string) => Promise<boolean>
  unlock: (username: string, password: string) => Promise<boolean>
  lock: () => void
  changePassword: (currentPassword: string, newPassword: string) => Promise<boolean>
  updatePasswordHint: (hint: string) => Promise<boolean>
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

  // Update actions
  checkForUpdates: () => Promise<void>
  downloadAndInstallUpdate: () => Promise<void>
  openReleasePage: () => Promise<void>
  dismissUpdateModal: () => void
  updateSettings: (settings: models.Settings) => Promise<boolean>

  // Computed
  filteredAccounts: () => models.Account[]
}

export const useAppStore = create<AppStore>((set, get) => ({
  // Initial state
  appState: 'loading',
  error: null,
  username: '',
  storedUsername: '',
  passwordHint: '',
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
  appVersion: '',
  updateInfo: null,
  isCheckingForUpdates: false,
  showUpdateModal: false,

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
        // Get stored username and password hint for login form
        try {
          const [storedUsername, passwordHint] = await Promise.all([
            GetStoredUsername(),
            GetPasswordHint(),
          ])
          set({ appState: 'locked', storedUsername, passwordHint: passwordHint || '' })
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
  createVault: async (username: string, password: string, hint?: string) => {
    try {
      if (hint) {
        await CreateVaultWithHint(username, password, hint)
      } else {
        await CreateVault(username, password)
      }
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

  // Change password
  changePassword: async (currentPassword: string, newPassword: string) => {
    try {
      await ChangePassword(currentPassword, newPassword)
      set({ error: null })
      return true
    } catch (e) {
      set({ error: String(e) })
      return false
    }
  },

  // Update password hint (for legacy users or changing hint)
  updatePasswordHint: async (hint: string) => {
    try {
      await UpdatePasswordHint(hint)
      set({ passwordHint: hint, error: null })
      return true
    } catch (e) {
      set({ error: String(e) })
      return false
    }
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

  // Update actions
  checkForUpdates: async () => {
    set({ isCheckingForUpdates: true })
    try {
      const version = await GetAppVersion()
      set({ appVersion: version })

      const info = await CheckForUpdates()
      if (info && info.available) {
        set({ updateInfo: info, showUpdateModal: true })
      } else {
        set({ updateInfo: info })
      }
    } catch (e) {
      console.error('Failed to check for updates:', e)
    } finally {
      set({ isCheckingForUpdates: false })
    }
  },

  downloadAndInstallUpdate: async () => {
    const { updateInfo } = get()
    if (!updateInfo?.downloadURL) return

    try {
      await DownloadAndInstallUpdate(updateInfo.downloadURL)
      // App will exit and installer will run
    } catch (e) {
      set({ error: `Update failed: ${e}` })
    }
  },

  openReleasePage: async () => {
    const { updateInfo } = get()
    if (!updateInfo?.releaseURL) return

    try {
      await OpenReleasePage(updateInfo.releaseURL)
    } catch (e) {
      console.error('Failed to open release page:', e)
    }
  },

  dismissUpdateModal: () => {
    set({ showUpdateModal: false })
  },

  updateSettings: async (settings: models.Settings) => {
    try {
      await UpdateSettings(settings)
      set({ settings, error: null })
      return true
    } catch (e) {
      set({ error: String(e) })
      return false
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
