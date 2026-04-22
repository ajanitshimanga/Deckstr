import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DeleteAccountModal } from '../AccountList'
import { models } from '../../../wailsjs/go/models'

function buildAccount(overrides: Partial<models.Account> = {}): models.Account {
  return {
    id: 'acct-1',
    displayName: 'Main Smurf',
    username: 'smurf123',
    password: 'hunter2',
    networkId: 'riot',
    tags: [],
    notes: '',
    riotId: '',
    region: 'na1',
    games: ['lol'],
    cachedRanks: [],
    puuid: '',
    ...overrides,
  } as models.Account
}

describe('DeleteAccountModal', () => {
  it('calls onConfirm when user clicks Delete', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()

    render(
      <DeleteAccountModal
        account={buildAccount()}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    )

    await user.click(screen.getByRole('button', { name: /^delete$/i }))

    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(onCancel).not.toHaveBeenCalled()
  })

  it('calls onCancel when user clicks Cancel', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()

    render(
      <DeleteAccountModal
        account={buildAccount()}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    )

    await user.click(screen.getByRole('button', { name: /^cancel$/i }))

    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('shows the account label in the confirmation copy', () => {
    render(
      <DeleteAccountModal
        account={buildAccount({ displayName: 'Main Smurf' })}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    )

    expect(screen.getByText('Main Smurf')).toBeInTheDocument()
  })
})
