import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// All store methods mocked — we only care that UnlockScreen calls the right
// store action with the right args for each user flow.

const createVault = vi.fn()
const unlock = vi.fn()
const resetPasswordWithRecoveryPhrase = vi.fn()
const clearError = vi.fn()
const adoptLegacyVault = vi.fn()

type LegacyVault = {
  path: string
  source: string
  username: string
  version: number
  sizeBytes: number
  modifiedAt: string
}

type Overrides = Partial<{
  appState: 'locked' | 'create' | 'unlocked' | 'loading'
  storedUsername: string
  passwordHint: string
  hasRecoveryPhrase: boolean
  error: string | null
  legacyVault: LegacyVault | null
  isAdoptingLegacy: boolean
  recentlyAdopted: boolean
}>

let current: Overrides = {}

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    appState: current.appState ?? 'locked',
    storedUsername: current.storedUsername ?? '',
    passwordHint: current.passwordHint ?? '',
    hasRecoveryPhrase: current.hasRecoveryPhrase ?? true,
    legacyVault: current.legacyVault ?? null,
    isAdoptingLegacy: current.isAdoptingLegacy ?? false,
    recentlyAdopted: current.recentlyAdopted ?? false,
    adoptLegacyVault,
    createVault,
    unlock,
    resetPasswordWithRecoveryPhrase,
    error: current.error ?? null,
    clearError,
  }),
}))

// The recovery-phrase modal is not under test here and uses its own store hook.
vi.mock('../RecoveryPhraseModal', () => ({ RecoveryPhraseModal: () => null }))

// OpenVaultFolder is invoked from the dead-end / banner-error escape hatches.
const openVaultFolder = vi.fn()
vi.mock('../../../wailsjs/go/main/App', () => ({
  OpenVaultFolder: () => openVaultFolder(),
}))

import { UnlockScreen } from '../UnlockScreen'

function render_withOverrides(overrides: Overrides = {}) {
  current = overrides
  return render(<UnlockScreen />)
}

const sampleLegacyVault: LegacyVault = {
  path: 'C:/Users/x/AppData/Roaming/OpenSmurfManager/vault.osm',
  source: 'OpenSmurfManager',
  username: 'alice',
  version: 2,
  sizeBytes: 4666,
  modifiedAt: '2026-04-15T12:00:00Z',
}

describe('UnlockScreen — sign-in (locked state)', () => {
  beforeEach(() => {
    createVault.mockReset()
    unlock.mockReset()
    resetPasswordWithRecoveryPhrase.mockReset()
    clearError.mockReset()
    adoptLegacyVault.mockReset()
  })

  it('calls unlock() with the entered credentials', async () => {
    const user = userEvent.setup()
    unlock.mockResolvedValue(true)
    render_withOverrides({ appState: 'locked' })

    await user.type(screen.getByPlaceholderText('Enter username'), 'eren')
    await user.type(screen.getByPlaceholderText('Enter password'), 'hunter22')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    expect(unlock).toHaveBeenCalledWith('eren', 'hunter22')
  })

  it('pre-fills the username from storedUsername', () => {
    render_withOverrides({ appState: 'locked', storedUsername: 'returning-user' })

    expect(screen.getByPlaceholderText('Enter username')).toHaveValue('returning-user')
  })

  it('toggles password visibility when eye icon is clicked', async () => {
    const user = userEvent.setup()
    render_withOverrides({ appState: 'locked' })

    const pw = screen.getByPlaceholderText('Enter password') as HTMLInputElement
    expect(pw.type).toBe('password')

    // The eye/eye-off button is the only button adjacent to the password input
    // that isn't Sign In. Find it by its position in the DOM (no aria-label
    // in the source, so target via its icon parent).
    const toggle = pw.parentElement!.querySelector('button[type="button"]') as HTMLButtonElement
    await user.click(toggle)

    expect(pw.type).toBe('text')
  })

  it('does not call unlock when the username is shorter than 3 chars', async () => {
    const user = userEvent.setup()
    render_withOverrides({ appState: 'locked' })

    await user.type(screen.getByPlaceholderText('Enter username'), 'ab')
    await user.type(screen.getByPlaceholderText('Enter password'), 'hunter22')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    expect(unlock).not.toHaveBeenCalled()
  })
})

describe('UnlockScreen — create-vault state', () => {
  beforeEach(() => {
    createVault.mockReset()
    adoptLegacyVault.mockReset()
  })

  it('submit button reads "Create Account" when appState is create', () => {
    render_withOverrides({ appState: 'create' })
    expect(screen.getByRole('button', { name: /create account/i })).toBeInTheDocument()
  })

  it('blocks submit when passwords do not match', async () => {
    const user = userEvent.setup()
    render_withOverrides({ appState: 'create' })

    await user.type(screen.getByPlaceholderText('Enter username'), 'brand-new')
    await user.type(screen.getByPlaceholderText('Enter password'), 'hunter22')
    // The confirm password field exists in create mode — match it by placeholder.
    const confirm = screen.getByPlaceholderText(/confirm/i)
    await user.type(confirm, 'mismatch22')

    await user.click(screen.getByRole('button', { name: /create account/i }))

    expect(createVault).not.toHaveBeenCalled()
  })

  it('shows the legacy-vault banner above the create form when an orphaned vault exists', () => {
    render_withOverrides({ appState: 'create', legacyVault: sampleLegacyVault })
    expect(screen.getByTestId('legacy-vault-banner')).toBeInTheDocument()
    // Compact banner shows the username inline plus the create-mode subtitle.
    expect(screen.getByText(/alice/)).toBeInTheDocument()
    expect(screen.getByText(/use it instead of starting over/i)).toBeInTheDocument()
  })
})

describe('UnlockScreen — recovery mode', () => {
  beforeEach(() => {
    resetPasswordWithRecoveryPhrase.mockReset()
    adoptLegacyVault.mockReset()
  })

  it('shows the "Forgot your password?" link by default even when hasRecoveryPhrase is false', () => {
    render_withOverrides({ appState: 'locked', hasRecoveryPhrase: false })
    // The lightweight link must be visible without the user needing to fail
    // three sign-in attempts first — that's the UX the user explicitly asked
    // for.
    expect(screen.getByRole('button', { name: /forgot your password\?/i })).toBeInTheDocument()
  })

  it('shows the link by default when hasRecoveryPhrase is true', () => {
    render_withOverrides({ appState: 'locked', hasRecoveryPhrase: true })
    expect(screen.getByRole('button', { name: /forgot your password\?/i })).toBeInTheDocument()
  })

  it('toggles into recovery mode when the "Forgot your password?" link is clicked', async () => {
    const user = userEvent.setup()
    render_withOverrides({ appState: 'locked', hasRecoveryPhrase: true })

    await user.click(screen.getByRole('button', { name: /forgot your password\?/i }))

    expect(screen.getByText(/enter your 6-word recovery phrase/i)).toBeInTheDocument()
  })

  it('in recovery mode without a current recovery phrase, surfaces the legacy vault adoption flow', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      appState: 'locked',
      hasRecoveryPhrase: false,
      legacyVault: sampleLegacyVault,
    })

    await user.click(screen.getByRole('button', { name: /forgot your password\?/i }))

    // Header swaps to the friendlier copy.
    expect(screen.getByRole('heading', { name: /forgot your password\?/i })).toBeInTheDocument()
    // The legacy banner shows up so the user can adopt the orphaned vault.
    expect(screen.getByTestId('legacy-vault-banner')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /use this vault/i })).toBeInTheDocument()
  })

  it('in recovery mode with no recovery phrase AND no legacy vault, explains there is no path forward', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      appState: 'locked',
      hasRecoveryPhrase: false,
      legacyVault: null,
    })

    await user.click(screen.getByRole('button', { name: /forgot your password\?/i }))

    expect(screen.getByText(/no recovery pin was saved/i)).toBeInTheDocument()
    // No recovery word inputs — we should not be in the 6-word entry form.
    expect(screen.queryByText(/enter your 6-word recovery phrase/i)).not.toBeInTheDocument()
  })
})

describe('UnlockScreen — legacy vault banner', () => {
  beforeEach(() => {
    adoptLegacyVault.mockReset()
  })

  it('renders the banner on the unlock screen when legacyVault is present', () => {
    render_withOverrides({ appState: 'locked', legacyVault: sampleLegacyVault })
    expect(screen.getByTestId('legacy-vault-banner')).toBeInTheDocument()
    // Compact banner: source + sign-in subtitle visible inline.
    expect(screen.getByText(/from opensmurfmanager/i)).toBeInTheDocument()
    expect(screen.getByText(/sign in with your old credentials/i)).toBeInTheDocument()
  })

  it('does NOT render the banner when there is no orphaned vault', () => {
    render_withOverrides({ appState: 'locked', legacyVault: null })
    expect(screen.queryByTestId('legacy-vault-banner')).not.toBeInTheDocument()
  })

  it('the primary "Use this vault" button does NOT immediately adopt — it opens a confirm panel', async () => {
    const user = userEvent.setup()
    adoptLegacyVault.mockResolvedValue(true)
    render_withOverrides({ appState: 'locked', legacyVault: sampleLegacyVault })

    await user.click(screen.getByRole('button', { name: /use this vault/i }))

    // No adoption call yet — we're in confirm mode.
    expect(adoptLegacyVault).not.toHaveBeenCalled()
    // Confirmation copy is visible so the user can see what's about to happen.
    expect(screen.getByText(/when you switch:/i)).toBeInTheDocument()
    expect(screen.getByText(/your current vault is archived/i)).toBeInTheDocument()
    expect(screen.getByText(/the opensmurfmanager folder is removed/i)).toBeInTheDocument()
  })

  it('clicking "Confirm switch" runs adoption', async () => {
    const user = userEvent.setup()
    adoptLegacyVault.mockResolvedValue(true)
    render_withOverrides({ appState: 'locked', legacyVault: sampleLegacyVault })

    await user.click(screen.getByRole('button', { name: /use this vault/i }))
    await user.click(screen.getByRole('button', { name: /confirm switch/i }))

    expect(adoptLegacyVault).toHaveBeenCalledTimes(1)
  })

  it('clicking "Cancel" returns to the compact banner without adopting', async () => {
    const user = userEvent.setup()
    render_withOverrides({ appState: 'locked', legacyVault: sampleLegacyVault })

    await user.click(screen.getByRole('button', { name: /use this vault/i }))
    expect(screen.getByText(/when you switch:/i)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /cancel/i }))

    // Confirm panel is gone, primary button is back.
    expect(screen.queryByText(/when you switch:/i)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /use this vault/i })).toBeInTheDocument()
    expect(adoptLegacyVault).not.toHaveBeenCalled()
  })

  it('disables the confirm button while adoption is in progress', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      appState: 'locked',
      legacyVault: sampleLegacyVault,
      isAdoptingLegacy: true,
    })

    // Open the confirm panel; the confirm button should be in "Switching…"
    // state and disabled while the parent reports isAdoptingLegacy.
    await user.click(screen.getByRole('button', { name: /use this vault/i }))
    const btn = screen.getByRole('button', { name: /switching/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
  })

  it('shows the "Show me my vault folder" escape hatch when adoption errors out', async () => {
    const user = userEvent.setup()
    openVaultFolder.mockReset()
    render_withOverrides({
      appState: 'locked',
      legacyVault: sampleLegacyVault,
      error: 'permission denied',
    })

    // The error message threads through into the banner so the user sees
    // why adoption failed and can fall back to a manual recovery. The
    // generic form-level error block also surfaces it — both paths are
    // valid, so just assert at least one match is on screen.
    expect(screen.getAllByText(/permission denied/i).length).toBeGreaterThan(0)

    const folderBtn = screen.getByRole('button', { name: /show me my vault folder/i })
    await user.click(folderBtn)

    expect(openVaultFolder).toHaveBeenCalledTimes(1)
  })
})

describe('UnlockScreen — post-adoption confirmation', () => {
  it('shows the green confirmation banner after a successful adoption', () => {
    render_withOverrides({
      appState: 'locked',
      legacyVault: null, // legacy folder is gone after adoption
      recentlyAdopted: true,
    })
    expect(screen.getByTestId('adopted-confirmation-banner')).toBeInTheDocument()
    expect(screen.getByText(/migrated to deckstr/i)).toBeInTheDocument()
    // The copy must explicitly tell the user OSM is gone — that's the
    // whole reason this banner exists.
    expect(screen.getByText(/opensmurfmanager has been removed/i)).toBeInTheDocument()
  })

  it('does NOT show the confirmation when an orphan still exists (legacyVault non-null)', () => {
    render_withOverrides({
      appState: 'locked',
      legacyVault: sampleLegacyVault,
      recentlyAdopted: true, // shouldn't matter — orphan supersedes
    })
    // The legacy banner takes precedence; we don't double-banner.
    expect(screen.getByTestId('legacy-vault-banner')).toBeInTheDocument()
    expect(screen.queryByTestId('adopted-confirmation-banner')).not.toBeInTheDocument()
  })

  it('does NOT show the confirmation when recentlyAdopted is false (the default)', () => {
    render_withOverrides({ appState: 'locked' })
    expect(screen.queryByTestId('adopted-confirmation-banner')).not.toBeInTheDocument()
  })
})

describe('UnlockScreen — recovery dead-end escape hatch', () => {
  beforeEach(() => {
    openVaultFolder.mockReset()
  })

  it('shows the "Show me my vault folder" link when there is no PIN and no legacy vault', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      appState: 'locked',
      hasRecoveryPhrase: false,
      legacyVault: null,
    })

    await user.click(screen.getByRole('button', { name: /forgot your password\?/i }))

    const folderBtn = screen.getByRole('button', { name: /show me my vault folder/i })
    await user.click(folderBtn)

    expect(openVaultFolder).toHaveBeenCalledTimes(1)
  })

  it('points users at Settings → Security → Import vault from file when stuck', async () => {
    const user = userEvent.setup()
    render_withOverrides({
      appState: 'locked',
      hasRecoveryPhrase: false,
      legacyVault: null,
    })

    await user.click(screen.getByRole('button', { name: /forgot your password\?/i }))
    expect(screen.getByText(/Settings → Security → Import vault from file/i)).toBeInTheDocument()
  })
})
