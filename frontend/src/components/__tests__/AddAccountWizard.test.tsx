import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

const addAccount = vi.fn()

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
    addAccount,
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
})
