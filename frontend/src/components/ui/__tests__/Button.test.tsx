import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from '../Button'

// AudioContext isn't in jsdom, so Button's click-sound call harmlessly no-ops.
// These tests focus on semantics: variant→class mapping, disabled handling,
// event propagation.

describe('Button primitive', () => {
  it('fires onClick when clicked', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Go</Button>)

    await user.click(screen.getByRole('button', { name: /go/i }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('does not fire onClick when disabled', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(
      <Button onClick={onClick} disabled>
        Go
      </Button>,
    )

    await user.click(screen.getByRole('button', { name: /go/i }))
    expect(onClick).not.toHaveBeenCalled()
  })

  it('renders leadingIcon + trailingIcon alongside children', () => {
    render(
      <Button
        leadingIcon={<span data-testid="lead">L</span>}
        trailingIcon={<span data-testid="trail">T</span>}
      >
        Mid
      </Button>,
    )

    expect(screen.getByTestId('lead')).toBeInTheDocument()
    expect(screen.getByTestId('trail')).toBeInTheDocument()
    expect(screen.getByRole('button')).toHaveTextContent(/LMidT/)
  })

  it('applies the destructive variant class for destructive buttons', () => {
    render(<Button variant="destructive">Delete</Button>)
    const btn = screen.getByRole('button', { name: /delete/i })
    // Destructive uses a red-tinted background utility — assert by substring.
    expect(btn.className).toMatch(/bg-red-500/)
  })

  it('stretches full-width when fullWidth prop is set', () => {
    render(<Button fullWidth>Wide</Button>)
    expect(screen.getByRole('button', { name: /wide/i }).className).toMatch(/w-full/)
  })

  it('uses a type="submit" button when passed type prop', () => {
    render(<Button type="submit">Submit</Button>)
    expect(screen.getByRole('button', { name: /submit/i })).toHaveAttribute('type', 'submit')
  })
})
