import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '../Modal'

describe('Modal primitive', () => {
  it('calls onClose when Escape is pressed', async () => {
    const onClose = vi.fn()
    render(
      <Modal onClose={onClose}>
        <ModalBody>content</ModalBody>
      </Modal>,
    )

    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when the backdrop is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const { container } = render(
      <Modal onClose={onClose}>
        <ModalBody>content</ModalBody>
      </Modal>,
    )

    // Backdrop is the outer wrapper with fixed inset — clicking it dismisses.
    const backdrop = container.querySelector('div.fixed.inset-0') as HTMLElement
    expect(backdrop).not.toBeNull()
    await user.click(backdrop)

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not dismiss on backdrop click when dismissOnBackdrop is false', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const { container } = render(
      <Modal onClose={onClose} dismissOnBackdrop={false}>
        <ModalBody>content</ModalBody>
      </Modal>,
    )

    const backdrop = container.querySelector('div.fixed.inset-0') as HTMLElement
    await user.click(backdrop)

    expect(onClose).not.toHaveBeenCalled()
  })

  it('does not dismiss when a click happens inside the modal body', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <Modal onClose={onClose}>
        <ModalBody>
          <button>inner</button>
        </ModalBody>
      </Modal>,
    )

    await user.click(screen.getByRole('button', { name: /inner/i }))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('locks body scroll while open and restores it on unmount', () => {
    const prev = document.body.style.overflow
    const { unmount } = render(
      <Modal onClose={() => {}}>
        <ModalBody>content</ModalBody>
      </Modal>,
    )

    expect(document.body.style.overflow).toBe('hidden')
    unmount()
    expect(document.body.style.overflow).toBe(prev)
  })

  it('renders header / body / footer content in order', () => {
    render(
      <Modal onClose={() => {}}>
        <ModalHeader>
          <h2>Title</h2>
        </ModalHeader>
        <ModalBody>Body text</ModalBody>
        <ModalFooter>
          <button>OK</button>
        </ModalFooter>
      </Modal>,
    )

    expect(screen.getByText('Title')).toBeInTheDocument()
    expect(screen.getByText('Body text')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /ok/i })).toBeInTheDocument()
  })
})
