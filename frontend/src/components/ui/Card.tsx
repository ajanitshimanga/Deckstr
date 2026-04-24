import { forwardRef } from 'react'
import type { HTMLAttributes, ReactNode } from 'react'
import { cn } from '../../lib/utils'
import { MOTION_BASE, MOTION_FOCUS } from '../../lib/motion'
import { playHover } from '../../lib/sound'

// Elevated surface. The three-tier elevation language:
//   - flat    : static content, slight border only
//   - raised  : default — subtle shadow + top-edge highlight
//   - elevated: for active / featured surfaces (e.g. signed-in account)
// When `interactive` is set, the card lifts on hover, fires a hover sound,
// and presses down on click — same vocabulary as Button/Tile.

type Elevation = 'flat' | 'raised' | 'elevated'

interface Props extends HTMLAttributes<HTMLDivElement> {
  elevation?: Elevation
  interactive?: boolean
  active?: boolean
  children?: ReactNode
}

const ELEVATION_CLASSES: Record<Elevation, string> = {
  flat: 'bg-[var(--color-card)] border border-[var(--color-border)]/40',
  raised: cn(
    'bg-[var(--color-card)]',
    'border border-[var(--color-border)]/50',
    'shadow-sm',
    // A 1px white top highlight fakes a light source from above — gives a
    // subtle "object in a scene" feel instead of a flat sticker.
    'before:absolute before:inset-x-0 before:top-0 before:h-px before:bg-white/[0.04] before:pointer-events-none',
  ),
  elevated: cn(
    'bg-[var(--color-card)]',
    'border border-[var(--color-border)]/60',
    'shadow-lg shadow-black/20',
    'before:absolute before:inset-x-0 before:top-0 before:h-px before:bg-white/[0.06] before:pointer-events-none',
  ),
}

export const Card = forwardRef<HTMLDivElement, Props>(function Card(
  { elevation = 'raised', interactive, active, className, onMouseEnter, children, ...rest },
  ref,
) {
  return (
    <div
      ref={ref}
      onMouseEnter={(e) => {
        if (interactive) playHover()
        onMouseEnter?.(e)
      }}
      className={cn(
        'relative rounded-xl overflow-hidden',
        ELEVATION_CLASSES[elevation],
        MOTION_BASE,
        interactive && cn(
          MOTION_FOCUS,
          'hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/25',
          'hover:border-[var(--color-border)]',
          'motion-reduce:hover:translate-y-0',
          'cursor-default',
        ),
        active && cn(
          'border-green-500/40 shadow-lg shadow-green-500/10',
          'before:bg-green-400/20',
        ),
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  )
})
