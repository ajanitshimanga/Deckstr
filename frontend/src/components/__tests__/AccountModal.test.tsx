import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const addAccount = vi.fn()
const editAccount = vi.fn()
const createTag = vi.fn()

vi.mock('../../stores/appStore', () => ({
  useAppStore: () => ({
    gameNetworks: [{ id: 'riot', name: 'Riot' }],
    tags: [],
    addAccount,
    editAccount,
    createTag,
  }),
}))

import { AccountModal } from '../AccountList'

describe('AccountModal — add account flow', () => {
  beforeEach(() => {
    addAccount.mockReset()
    editAccount.mockReset()
    createTag.mockReset()
  })

  it('submits new account when user clicks Add', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()

    render(<AccountModal account={null} onClose={onClose} />)

    await user.type(screen.getByPlaceholderText('Enter username'), 'testuser')
    await user.type(screen.getByPlaceholderText('Enter password'), 'testpass')

    const submitButton = screen
      .getAllByRole('button', { name: /^add$/i })
      .find((b) => (b as HTMLButtonElement).type === 'submit') as HTMLButtonElement
    expect(submitButton).toBeDefined()
    await user.click(submitButton)

    expect(addAccount).toHaveBeenCalledTimes(1)
    expect(addAccount).toHaveBeenCalledWith(
      expect.objectContaining({ username: 'testuser', password: 'testpass' }),
    )
    expect(onClose).toHaveBeenCalled()
  })

  it('keeps submit button wired to the form (regression: detached </form>)', () => {
    render(<AccountModal account={null} onClose={vi.fn()} />)

    const submitButton = screen
      .getAllByRole('button', { name: /^add$/i })
      .find((b) => (b as HTMLButtonElement).type === 'submit') as HTMLButtonElement
    // The button must either be inside a <form> or explicitly associated via `form` attribute.
    // Without this, clicking "Add" is a no-op and customers cannot create accounts.
    const associatedForm = submitButton.form
    expect(associatedForm).not.toBeNull()
    expect(associatedForm?.tagName).toBe('FORM')
  })
})
