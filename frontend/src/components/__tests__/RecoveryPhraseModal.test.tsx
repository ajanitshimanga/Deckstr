import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const dismissRecoveryPhraseModal = vi.fn()

type StoreOverrides = Partial<{
  showRecoveryPhraseModal: boolean
  pendingRecoveryPhrase: string | null
  requiresPinSetup: boolean
}>

let currentOverrides: StoreOverrides = {}

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    showRecoveryPhraseModal: currentOverrides.showRecoveryPhraseModal ?? true,
    // Check "has own property" so an explicit `pendingRecoveryPhrase: null`
    // passes through rather than falling back to the default string.
    pendingRecoveryPhrase: Object.prototype.hasOwnProperty.call(
      currentOverrides,
      'pendingRecoveryPhrase',
    )
      ? currentOverrides.pendingRecoveryPhrase
      : 'alpha beta gamma delta epsilon zeta',
    requiresPinSetup: currentOverrides.requiresPinSetup ?? true,
    dismissRecoveryPhraseModal,
  }),
}))

import { RecoveryPhraseModal } from '../RecoveryPhraseModal'

function render_withOverrides(overrides: StoreOverrides = {}) {
  currentOverrides = overrides
  return render(<RecoveryPhraseModal />)
}

describe('RecoveryPhraseModal', () => {
  beforeEach(() => {
    dismissRecoveryPhraseModal.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders nothing when the modal flag is off', () => {
    const { container } = render_withOverrides({ showRecoveryPhraseModal: false })
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when there is no pending phrase', () => {
    const { container } = render_withOverrides({ pendingRecoveryPhrase: null })
    expect(container.firstChild).toBeNull()
  })

  it('starts with the reveal CTA and no "Continue" button', () => {
    render_withOverrides()

    expect(screen.getByRole('button', { name: /click to reveal pin/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /saved my recovery pin/i })).not.toBeInTheDocument()
    // The individual phrase words should not be visible yet — the reveal UI
    // shows dots placeholder.
    expect(screen.queryByText('alpha')).not.toBeInTheDocument()
  })

  it('reveals the six phrase words after the shimmer timer elapses', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    render_withOverrides()

    await user.click(screen.getByRole('button', { name: /click to reveal pin/i }))

    // Shimmer is 1 second then setShowPhrase(true)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100)
    })

    expect(screen.getByText('alpha')).toBeInTheDocument()
    expect(screen.getByText('zeta')).toBeInTheDocument()
    // Continue button becomes available only after reveal.
    expect(screen.getByRole('button', { name: /saved my recovery pin/i })).toBeInTheDocument()
  })

  it('calls dismissRecoveryPhraseModal when the user confirms saved', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    render_withOverrides()

    await user.click(screen.getByRole('button', { name: /click to reveal pin/i }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100)
    })

    await user.click(screen.getByRole('button', { name: /saved my recovery pin/i }))

    expect(dismissRecoveryPhraseModal).toHaveBeenCalledTimes(1)
  })

  it('copies the phrase to clipboard when the copy button is clicked post-reveal', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    // navigator.clipboard is a read-only accessor — defineProperty lets us
    // swap in a spy that the component can call normally.
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
      writable: true,
    })

    render_withOverrides()

    await user.click(screen.getByRole('button', { name: /click to reveal pin/i }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100)
    })

    await user.click(screen.getByRole('button', { name: /^copy$/i }))

    expect(writeText).toHaveBeenCalledWith('alpha beta gamma delta epsilon zeta')
  })
})
