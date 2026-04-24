import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { cn } from '../../lib/utils'

// Shared modal shell. Handles:
//   - backdrop fade-in
//   - content scale+slide entry
//   - Esc to dismiss
//   - scroll lock on <body> while open
// Consumers provide header / body / footer — we don't over-prescribe shape.

interface Props {
  onClose: () => void
  size?: 'sm' | 'md' | 'lg'
  dismissOnBackdrop?: boolean
  className?: string
  children: ReactNode
}

const SIZE_CLASSES: Record<NonNullable<Props['size']>, string> = {
  sm: 'max-w-[95%] sm:max-w-sm',
  md: 'max-w-[95%] sm:max-w-md',
  lg: 'max-w-[95%] sm:max-w-lg',
}

export function Modal({ onClose, size = 'md', dismissOnBackdrop = true, className, children }: Props) {
  // Lock body scroll while open — prevents the list behind the modal from
  // scrolling when the user hits space/arrow keys.
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = prev
      window.removeEventListener('keydown', onKey)
    }
  }, [onClose])

  return (
    <div
      className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 z-50 animate-fade-in"
      onClick={dismissOnBackdrop ? onClose : undefined}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className={cn(
          'w-full bg-[var(--color-card)] rounded-xl sm:rounded-2xl',
          'border border-[var(--color-border)]',
          'shadow-2xl shadow-black/40',
          'overflow-hidden max-h-[90vh] flex flex-col',
          // Top-edge highlight — echoes the Card raised elevation language.
          'relative before:absolute before:inset-x-0 before:top-0 before:h-px before:bg-white/[0.06] before:pointer-events-none',
          // Scale up from 96% + slide up 8px over 180ms with spring-ish easing.
          'animate-scale-in',
          SIZE_CLASSES[size],
          className,
        )}
      >
        {children}
      </div>
    </div>
  )
}

// Convenience sub-shells for consistent padding/borders.
export function ModalHeader({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('p-3 sm:p-4 border-b border-[var(--color-border)]/70 shrink-0', className)}>
      {children}
    </div>
  )
}

export function ModalBody({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('p-3 sm:p-4 overflow-y-auto flex-1', className)}>{children}</div>
  )
}

export function ModalFooter({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'p-3 sm:p-4 border-t border-[var(--color-border)]/70 shrink-0 flex gap-2 sm:gap-3',
        className,
      )}
    >
      {children}
    </div>
  )
}
