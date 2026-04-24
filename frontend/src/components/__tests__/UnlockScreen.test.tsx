import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// All store methods mocked — we only care that UnlockScreen calls the right
// store action with the right args for each user flow.

const createVault = vi.fn()
const unlock = vi.fn()
const resetPasswordWithRecoveryPhrase = vi.fn()
const clearError = vi.fn()

type Overrides = Partial<{
  appState: 'locked' | 'create' | 'unlocked' | 'loading'
  storedUsername: string
  passwordHint: string
  hasRecoveryPhrase: boolean
  error: string | null
}>

let current: Overrides = {}

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    appState: current.appState ?? 'locked',
    storedUsername: current.storedUsername ?? '',
    passwordHint: current.passwordHint ?? '',
    hasRecoveryPhrase: current.hasRecoveryPhrase ?? true,
    createVault,
    unlock,
    resetPasswordWithRecoveryPhrase,
    error: current.error ?? null,
    clearError,
  }),
}))

// The recovery-phrase modal is not under test here and uses its own store hook.
vi.mock('../RecoveryPhraseModal', () => ({ RecoveryPhraseModal: () => null }))

import { UnlockScreen } from '../UnlockScreen'

function render_withOverrides(overrides: Overrides = {}) {
  current = overrides
  return render(<UnlockScreen />)
}

describe('UnlockScreen — sign-in (locked state)', () => {
  beforeEach(() => {
    createVault.mockReset()
    unlock.mockReset()
    resetPasswordWithRecoveryPhrase.mockReset()
    clearError.mockReset()
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
})

describe('UnlockScreen — recovery mode', () => {
  beforeEach(() => {
    resetPasswordWithRecoveryPhrase.mockReset()
  })

  it('toggles into recovery mode when the "Use recovery PIN" link is clicked', async () => {
    const user = userEvent.setup()
    render_withOverrides({ appState: 'locked', hasRecoveryPhrase: true })

    // The recovery link copy lives inline — assert it's present before clicking.
    const link = screen.getByText(/forgot password\? use recovery pin/i)
    await user.click(link)

    expect(screen.getByText(/enter your 6-word recovery phrase/i)).toBeInTheDocument()
  })
})
