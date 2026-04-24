import { forwardRef } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { cn } from '../../lib/utils'
import { MOTION_BASE, MOTION_FOCUS } from '../../lib/motion'

// Compact, icon-only button. Used for per-card actions (edit, delete, copy,
// eye). Lighter than Button — no lift, just a subtle bg swap on hover.

type Tone = 'neutral' | 'destructive' | 'success' | 'brand'
type Size = 'sm' | 'md'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: ReactNode
  tone?: Tone
  size?: Size
  ariaLabel: string
}

const TONE_CLASSES: Record<Tone, string> = {
  neutral: cn(
    'text-[var(--color-muted-foreground)]',
    'hover:text-[var(--color-foreground)]',
    'hover:bg-[var(--color-muted)]/60',
  ),
  destructive: cn(
    'text-[var(--color-muted-foreground)]',
    'hover:text-red-400',
    'hover:bg-red-500/10',
  ),
  success: cn(
    'text-green-400',
    'bg-green-500/10',
    'hover:bg-green-500/20',
  ),
  brand: cn(
    'text-[var(--color-primary)]',
    'hover:bg-[var(--color-primary)]/10',
  ),
}

const SIZE_CLASSES: Record<Size, string> = {
  sm: 'w-7 h-7 rounded-md',
  md: 'w-8 h-8 rounded-lg',
}

export const IconButton = forwardRef<HTMLButtonElement, Props>(function IconButton(
  {
    icon,
    tone = 'neutral',
    size = 'md',
    ariaLabel,
    className,
    onClick,
    onMouseEnter,
    disabled,
    ...rest
  },
  ref,
) {
  return (
    <button
      ref={ref}
      aria-label={ariaLabel}
      disabled={disabled}
      onMouseEnter={onMouseEnter}
      onClick={onClick}
      className={cn(
        'inline-flex items-center justify-center',
        MOTION_BASE,
        MOTION_FOCUS,
        'enabled:active:scale-[0.92] motion-reduce:enabled:active:scale-100',
        'disabled:opacity-50 disabled:cursor-not-allowed',
        TONE_CLASSES[tone],
        SIZE_CLASSES[size],
        className,
      )}
      {...rest}
    >
      {icon}
    </button>
  )
})
