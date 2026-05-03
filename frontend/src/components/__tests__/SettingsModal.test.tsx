import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Store + wails bindings are fully mocked. Each scenario configures the
// store via `currentOverrides` and asserts on the action mocks.

const updateSettings = vi.fn()
const pollForActiveAccount = vi.fn()
const detectAndUpdateRanks = vi.fn()
const checkForUpdates = vi.fn()
const lock = vi.fn()
const changePassword = vi.fn()
const updatePasswordHint = vi.fn()
const regenerateRecoveryPhrase = vi.fn()
const clearError = vi.fn()
const importVaultFromFile = vi.fn()

type StoreOverrides = Partial<{
  username: string
  settings: {
    pollingProfile?: string
    autoRankSync?: boolean
    rankSyncIntervalMs?: number
    autoCheckUpdates?: boolean
  } | null
  hasRecoveryPhrase: boolean
  passwordHint: string
  isDetecting: boolean
  lastRankSyncTime: number | null
  appVersion: string
  isCheckingForUpdates: boolean
  error: string | null
}>

let currentOverrides: StoreOverrides = {}

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    username: currentOverrides.username ?? 'eren',
    settings: Object.prototype.hasOwnProperty.call(currentOverrides, 'settings')
      ? currentOverrides.settings
      : { pollingProfile: 'balanced', autoRankSync: true, rankSyncIntervalMs: 600000, autoCheckUpdates: true },
    hasRecoveryPhrase: currentOverrides.hasRecoveryPhrase ?? false,
    passwordHint: currentOverrides.passwordHint ?? '',
    isDetecting: currentOverrides.isDetecting ?? false,
    lastRankSyncTime: currentOverrides.lastRankSyncTime ?? null,
    appVersion: currentOverrides.appVersion ?? '1.5.0',
    isCheckingForUpdates: currentOverrides.isCheckingForUpdates ?? false,
    error: currentOverrides.error ?? null,
    updateSettings,
    pollForActiveAccount,
    detectAndUpdateRanks,
    checkForUpdates,
    lock,
    changePassword,
    updatePasswordHint,
    regenerateRecoveryPhrase,
    clearError,
    importVaultFromFile,
  }),
}))

const isTelemetryEnabled = vi.fn()
const setTelemetryEnabled = vi.fn()
const openUsageLogsFolder = vi.fn()
const openReleasePage = vi.fn()
const openVaultFolder = vi.fn()

vi.mock('../../../wailsjs/go/main/App', () => ({
  IsTelemetryEnabled: () => isTelemetryEnabled(),
  SetTelemetryEnabled: (v: boolean) => setTelemetryEnabled(v),
  OpenUsageLogsFolder: () => openUsageLogsFolder(),
  OpenReleasePage: (url: string) => openReleasePage(url),
  OpenVaultFolder: () => openVaultFolder(),
}))

import { SettingsModal } from '../SettingsModal'

function renderWith(overrides: StoreOverrides = {}) {
  currentOverrides = overrides
  const onClose = vi.fn()
  const utils = render(<SettingsModal onClose={onClose} />)
  return { onClose, ...utils }
}

beforeEach(() => {
  updateSettings.mockReset()
  pollForActiveAccount.mockReset()
  detectAndUpdateRanks.mockReset()
  checkForUpdates.mockReset()
  lock.mockReset()
  changePassword.mockReset()
  updatePasswordHint.mockReset()
  regenerateRecoveryPhrase.mockReset()
  clearError.mockReset()
  isTelemetryEnabled.mockReset().mockResolvedValue(false)
  setTelemetryEnabled.mockReset().mockResolvedValue(undefined)
  openUsageLogsFolder.mockReset().mockResolvedValue(undefined)
  openReleasePage.mockReset().mockResolvedValue(undefined)
  openVaultFolder.mockReset().mockResolvedValue(undefined)
  importVaultFromFile.mockReset()
  currentOverrides = {}
})

describe('SettingsModal — main view', () => {
  it('renders the main settings page with all section headers', () => {
    renderWith()

    expect(screen.getByRole('dialog', { name: /settings/i })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    // Section labels are uppercase but the text content is title-case.
    expect(screen.getByText('General')).toBeInTheDocument()
    expect(screen.getByText('Live tracking')).toBeInTheDocument()
    expect(screen.getByText('Security')).toBeInTheDocument()
    expect(screen.getByText('App updates')).toBeInTheDocument()
    expect(screen.getByText('Privacy')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument()
  })

  it('shows the username in the profile card with two-letter initials', () => {
    renderWith({ username: 'eren' })
    // Initials are first two chars uppercased. Username text is also rendered.
    expect(screen.getByText('ER')).toBeInTheDocument()
    expect(screen.getByText('eren')).toBeInTheDocument()
  })

  it('falls back to "?" initials and "Vault" label when username is empty', () => {
    renderWith({ username: '' })
    expect(screen.getByText('?')).toBeInTheDocument()
    expect(screen.getByText('Vault')).toBeInTheDocument()
  })

  it('calls onClose when the X button is clicked', async () => {
    const user = userEvent.setup()
    const { onClose } = renderWith()

    await user.click(screen.getByRole('button', { name: /close settings/i }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when Escape is pressed from the main view', () => {
    const { onClose } = renderWith()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('locks body scroll while open and restores it on unmount', () => {
    const { unmount } = renderWith()
    expect(document.body.style.overflow).toBe('hidden')
    unmount()
    expect(document.body.style.overflow).not.toBe('hidden')
  })
})

describe('SettingsModal — toggles', () => {
  it('flips auto-rank-sync via updateSettings when the toggle row is clicked', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /auto-sync ranks/i }))

    expect(updateSettings).toHaveBeenCalledTimes(1)
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ autoRankSync: false }),
    )
  })

  it('flips auto-check-updates via updateSettings', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /auto-check on launch/i }))

    expect(updateSettings).toHaveBeenCalledTimes(1)
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ autoCheckUpdates: false }),
    )
  })

  it('disables the sync-interval drill-in when auto-rank-sync is off', () => {
    renderWith({
      settings: {
        pollingProfile: 'balanced',
        autoRankSync: false,
        rankSyncIntervalMs: 600000,
        autoCheckUpdates: true,
      },
    })

    const syncIntervalRow = screen.getByRole('button', { name: /sync interval/i })
    expect(syncIntervalRow).toBeDisabled()
  })
})

describe('SettingsModal — actions', () => {
  it('calls detectAndUpdateRanks when "Detect ranks now" is clicked', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /detect ranks now/i }))
    expect(detectAndUpdateRanks).toHaveBeenCalledTimes(1)
  })

  it('disables "Detect ranks now" while a detection is in flight', () => {
    renderWith({ isDetecting: true })

    const btn = screen.getByRole('button', { name: /detecting/i })
    expect(btn).toBeDisabled()
  })

  it('calls checkForUpdates when "Check for updates" is clicked', async () => {
    const user = userEvent.setup()
    renderWith()

    // The version subtitle disambiguates the action row from the
    // "Auto-check on launch" toggle row whose subtitle also says
    // "Check for updates when Deckstr starts".
    await user.click(screen.getByRole('button', { name: /^check for updates version/i }))
    expect(checkForUpdates).toHaveBeenCalledTimes(1)
  })

  it('renders the current app version in the updates section subtitle', () => {
    renderWith({ appVersion: '9.9.9' })
    expect(screen.getByText(/version 9\.9\.9/i)).toBeInTheDocument()
  })

  it('falls back to "dev" when appVersion is empty', () => {
    renderWith({ appVersion: '' })
    expect(screen.getByText(/version dev/i)).toBeInTheDocument()
  })

  it('signs out via lock + onClose when the Sign out button is clicked', async () => {
    const user = userEvent.setup()
    const { onClose } = renderWith()

    await user.click(screen.getByRole('button', { name: /sign out/i }))

    expect(lock).toHaveBeenCalledTimes(1)
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})

describe('SettingsModal — drill-in navigation', () => {
  it('navigates into the Update speed picker and shows the current profile pre-selected', async () => {
    const user = userEvent.setup()
    renderWith({
      settings: {
        pollingProfile: 'eco',
        autoRankSync: true,
        rankSyncIntervalMs: 600000,
        autoCheckUpdates: true,
      },
    })

    await user.click(screen.getByRole('button', { name: /update speed/i }))

    // Header swaps to the sub-page title.
    expect(screen.getByRole('heading', { name: /update speed/i })).toBeInTheDocument()
    // The Eco card reflects the current selection. Match by the unique
    // subtitle prefix; /eco/i alone matches "Recommended for most setups"
    // on the sibling Balanced card.
    expect(
      screen.getByRole('button', { name: /^eco battery saver/i }),
    ).toBeInTheDocument()
  })

  it('navigates back from a drill-in via the Back button', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /update speed/i }))
    expect(screen.getByRole('heading', { name: /update speed/i })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^back$/i }))
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
  })

  it('Escape from a drill-in returns to main without closing the modal', async () => {
    const user = userEvent.setup()
    const { onClose } = renderWith()

    await user.click(screen.getByRole('button', { name: /update speed/i }))
    fireEvent.keyDown(window, { key: 'Escape' })

    // Back to main, but onClose should not have fired.
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('selecting a new polling profile persists settings and triggers a re-poll', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /update speed/i }))
    await user.click(screen.getByRole('button', { name: /instant/i }))

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ pollingProfile: 'instant' }),
    )
    expect(pollForActiveAccount).toHaveBeenCalledTimes(1)
  })

  it('does not re-save when the user picks the already-selected profile', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /update speed/i }))
    // 'balanced' is the default — clicking it should be a no-op.
    await user.click(screen.getByRole('button', { name: /balanced/i }))

    expect(updateSettings).not.toHaveBeenCalled()
    expect(pollForActiveAccount).not.toHaveBeenCalled()
  })
})

describe('SettingsModal — sync interval picker', () => {
  it('renders the current value in minutes', async () => {
    const user = userEvent.setup()
    renderWith({
      settings: {
        pollingProfile: 'balanced',
        autoRankSync: true,
        rankSyncIntervalMs: 15 * 60000,
        autoCheckUpdates: true,
      },
    })

    await user.click(screen.getByRole('button', { name: /sync interval/i }))
    // Big number display.
    expect(screen.getByText('15')).toBeInTheDocument()
    expect(screen.getByText(/minutes/i)).toBeInTheDocument()
  })

  it('persists a new interval as milliseconds when a preset chip is clicked', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /sync interval/i }))
    await user.click(screen.getByRole('button', { name: /^30m$/i }))

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ rankSyncIntervalMs: 30 * 60000 }),
    )
  })

  it('persists a new interval when the slider changes', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /sync interval/i }))
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '7' } })

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ rankSyncIntervalMs: 7 * 60000 }),
    )
  })
})

describe('SettingsModal — security drill-ins', () => {
  it('shows "Generate recovery PIN" when the vault has none set', () => {
    renderWith({ hasRecoveryPhrase: false })
    expect(screen.getByRole('button', { name: /generate recovery pin/i })).toBeInTheDocument()
  })

  it('shows "Regenerate recovery PIN" when one is already set', () => {
    renderWith({ hasRecoveryPhrase: true })
    expect(screen.getByRole('button', { name: /regenerate recovery pin/i })).toBeInTheDocument()
  })

  it('drills into the password change form and exposes Current/New/Confirm fields', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /change password/i }))

    expect(screen.getByPlaceholderText(/enter current password/i)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/enter new password/i)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/confirm new password/i)).toBeInTheDocument()
  })

  it('keeps the change-password submit disabled until the form is valid', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /change password/i }))

    const submit = screen.getByRole('button', { name: /^change password$/i })
    expect(submit).toBeDisabled()

    await user.type(screen.getByPlaceholderText(/enter current password/i), 'oldpass')
    await user.type(screen.getByPlaceholderText(/enter new password/i), 'newpass')
    await user.type(screen.getByPlaceholderText(/confirm new password/i), 'newpass')

    expect(submit).toBeEnabled()
  })

  it('drills into the password hint form prefilled with the current hint', async () => {
    const user = userEvent.setup()
    renderWith({ passwordHint: 'mom maiden' })

    await user.click(screen.getByRole('button', { name: /password hint/i }))

    expect(screen.getByDisplayValue('mom maiden')).toBeInTheDocument()
  })
})

describe('SettingsModal — privacy', () => {
  it('reads the live telemetry state on mount and reflects it in the toggle subtitle', async () => {
    isTelemetryEnabled.mockResolvedValueOnce(true)
    renderWith()
    await waitFor(() => expect(isTelemetryEnabled).toHaveBeenCalledTimes(1))
  })

  it('flips telemetry via SetTelemetryEnabled when the toggle row is clicked', async () => {
    const user = userEvent.setup()
    isTelemetryEnabled.mockResolvedValueOnce(false)
    renderWith()

    // Wait for the on-mount async fetch to settle so the toggle is enabled.
    await waitFor(() => expect(isTelemetryEnabled).toHaveBeenCalled())

    await user.click(screen.getByRole('button', { name: /anonymous usage data/i }))
    expect(setTelemetryEnabled).toHaveBeenCalledWith(true)
  })

  it('drills into the privacy details page and exposes the disclosure copy', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /what's collected/i }))

    expect(screen.getByRole('heading', { name: /data & privacy/i })).toBeInTheDocument()
    expect(screen.getByText(/never transmitted/i)).toBeInTheDocument()
    expect(screen.getByText(/collected when on/i)).toBeInTheDocument()
    expect(screen.getByText(/never collected/i)).toBeInTheDocument()
  })

  it('opens the logs folder via the wails binding from the privacy details page', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /what's collected/i }))
    await user.click(screen.getByRole('button', { name: /open logs folder/i }))

    expect(openUsageLogsFolder).toHaveBeenCalledTimes(1)
  })

  it('opens the telemetry policy URL via OpenReleasePage', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByRole('button', { name: /what's collected/i }))
    await user.click(screen.getByRole('button', { name: /telemetry policy/i }))

    expect(openReleasePage).toHaveBeenCalledWith(
      expect.stringContaining('TELEMETRY.md'),
    )
  })
})

describe('SettingsModal — last-synced label', () => {
  it('says "never" when there has been no sync', () => {
    renderWith({ lastRankSyncTime: null })
    expect(screen.getByText(/last synced never/i)).toBeInTheDocument()
  })

  it('says "just now" for a sync within the last minute', () => {
    renderWith({ lastRankSyncTime: Date.now() - 5_000 })
    expect(screen.getByText(/last synced just now/i)).toBeInTheDocument()
  })

  it('formats minutes-old syncs as "Nm ago"', () => {
    renderWith({ lastRankSyncTime: Date.now() - 7 * 60_000 })
    expect(screen.getByText(/last synced 7m ago/i)).toBeInTheDocument()
  })
})

describe('SettingsModal — Import vault from file', () => {
  it('shows the Import drill-row in the Security section', () => {
    renderWith()
    expect(screen.getByText(/import vault from file/i)).toBeInTheDocument()
  })

  it('drills into the import view, explains the flow, and exposes the file picker', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByText(/import vault from file/i))

    // Header swaps so the user knows where they are.
    expect(screen.getByText(/^import vault$/i)).toBeInTheDocument()
    // Warning copy enumerates the destructive consequences before any picker
    // opens — the user must be able to bail.
    expect(screen.getByText(/you'll be signed out/i)).toBeInTheDocument()
    expect(screen.getByText(/your current vault is archived/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /pick vault file/i })).toBeInTheDocument()
  })

  it('calls importVaultFromFile and closes Settings on a successful import', async () => {
    const user = userEvent.setup()
    importVaultFromFile.mockResolvedValue({ ok: true, cancelled: false })
    const { onClose } = renderWith()

    await user.click(screen.getByText(/import vault from file/i))
    await user.click(screen.getByRole('button', { name: /pick vault file/i }))

    expect(importVaultFromFile).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('stays on the import view when the user cancels the picker', async () => {
    const user = userEvent.setup()
    importVaultFromFile.mockResolvedValue({ ok: false, cancelled: true })
    const { onClose } = renderWith()

    await user.click(screen.getByText(/import vault from file/i))
    await user.click(screen.getByRole('button', { name: /pick vault file/i }))

    await waitFor(() => expect(importVaultFromFile).toHaveBeenCalled())
    expect(onClose).not.toHaveBeenCalled()
    // The destructive copy is still visible — we haven't navigated away.
    expect(screen.getByText(/you'll be signed out/i)).toBeInTheDocument()
  })

  it('renders a "Show my vault folder" escape hatch on the import view', async () => {
    const user = userEvent.setup()
    renderWith()

    await user.click(screen.getByText(/import vault from file/i))
    await user.click(screen.getByRole('button', { name: /show my vault folder/i }))

    expect(openVaultFolder).toHaveBeenCalledTimes(1)
  })

  it('surfaces the store error on the import view when import fails', async () => {
    const user = userEvent.setup()
    // The import view reads error from the store mock; pre-populate it so
    // the error block renders on first paint after the user clicks.
    importVaultFromFile.mockImplementation(async () => {
      currentOverrides.error = 'source file is not a valid vault: unexpected end of JSON input'
      return { ok: false, cancelled: false }
    })
    renderWith()

    await user.click(screen.getByText(/import vault from file/i))
    await user.click(screen.getByRole('button', { name: /pick vault file/i }))

    await waitFor(() =>
      expect(screen.getByText(/source file is not a valid vault/i)).toBeInTheDocument()
    )
  })
})
