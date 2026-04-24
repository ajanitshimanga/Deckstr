import { forwardRef } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { cn } from '../../lib/utils'
import { MOTION_BASE, MOTION_FOCUS } from '../../lib/motion'
import { playTick, playError, playPop } from '../../lib/sound'

// Shared button primitive. All app buttons route through here so the tactile
// language (hover lift, press, sound, focus ring) lives in one place.

type Variant = 'primary' | 'secondary' | 'ghost' | 'destructive'
type Size = 'sm' | 'md' | 'lg'
// Click sound vocabulary — callers pick intent, the primitive picks the tone.
//   tick  — default small commit (most buttons)
//   pop   — satisfying commit (add-something, select-a-tile)
//   error — destructive / cancel
//   none  — caller is playing its own sound in the onClick handler
type ClickSound = 'tick' | 'pop' | 'error' | 'none'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  leadingIcon?: ReactNode
  trailingIcon?: ReactNode
  fullWidth?: boolean
  // Deprecated shortcut for `clickSound="none"` + silenced hover — still
  // honoured for existing callers that just want the whole button quiet.
  sound?: boolean
  // Optional override for the click sound. When omitted, picks a sensible
  // default from variant (destructive→error, otherwise tick).
  clickSound?: ClickSound
}

const VARIANT_CLASSES: Record<Variant, string> = {
  primary: cn(
    'bg-[var(--color-primary)] text-white',
    'hover:bg-[var(--color-primary)]/90',
    'shadow-md shadow-[var(--color-primary)]/25',
    'hover:shadow-lg hover:shadow-[var(--color-primary)]/40',
    'disabled:opacity-50 disabled:cursor-not-allowed disabled:shadow-none',
  ),
  secondary: cn(
    'bg-[var(--color-muted)] text-[var(--color-foreground)]',
    'hover:bg-[var(--color-border)]',
    'border border-[var(--color-border)]/50',
    'hover:border-[var(--color-border)]',
    'disabled:opacity-50 disabled:cursor-not-allowed',
  ),
  ghost: cn(
    'bg-transparent text-[var(--color-muted-foreground)]',
    'hover:bg-[var(--color-muted)]/50 hover:text-[var(--color-foreground)]',
    'disabled:opacity-50 disabled:cursor-not-allowed',
  ),
  destructive: cn(
    'bg-red-500/90 text-white',
    'hover:bg-red-500',
    'shadow-md shadow-red-500/25 hover:shadow-lg hover:shadow-red-500/40',
    'disabled:opacity-50 disabled:cursor-not-allowed disabled:shadow-none',
  ),
}

const SIZE_CLASSES: Record<Size, string> = {
  sm: 'h-8 px-3 text-xs rounded-lg gap-1.5',
  md: 'h-10 px-4 text-sm rounded-xl gap-2',
  lg: 'h-11 px-5 text-sm rounded-xl gap-2',
}

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  {
    variant = 'primary',
    size = 'md',
    leadingIcon,
    trailingIcon,
    fullWidth,
    sound = true,
    clickSound,
    className,
    onClick,
    onMouseEnter,
    disabled,
    children,
    ...rest
  },
  ref,
) {
  const effectiveClickSound: ClickSound =
    clickSound ?? (sound ? (variant === 'destructive' ? 'error' : 'tick') : 'none')

  return (
    <button
      ref={ref}
      disabled={disabled}
      onMouseEnter={onMouseEnter}
      onClick={(e) => {
        if (!disabled) {
          switch (effectiveClickSound) {
            case 'tick':
              playTick()
              break
            case 'pop':
              playPop()
              break
            case 'error':
              playError()
              break
            case 'none':
              break
          }
        }
        onClick?.(e)
      }}
      className={cn(
        'inline-flex items-center justify-center font-medium select-none',
        MOTION_BASE,
        MOTION_FOCUS,
        // Lift + press — neutralized for disabled state via :not()
        'enabled:hover:-translate-y-0.5 enabled:active:translate-y-0 enabled:active:scale-[0.98]',
        'motion-reduce:enabled:hover:translate-y-0 motion-reduce:enabled:active:scale-100',
        VARIANT_CLASSES[variant],
        SIZE_CLASSES[size],
        fullWidth && 'w-full',
        className,
      )}
      {...rest}
    >
      {leadingIcon}
      {children}
      {trailingIcon}
    </button>
  )
})
