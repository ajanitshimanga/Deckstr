// Shared motion vocabulary. Centralising these classes means we can tweak the
// feel in one place and keep "the app" coherent instead of every component
// inventing its own easing.
//
// Principles:
//   - 150ms for most UI transitions (fast enough to feel instant, slow enough
//     to register).
//   - ease-out for entries, ease-in for exits. Things "settle" on arrival.
//   - Hover lifts are small (1px) — the goal is acknowledgement, not theatre.
//   - Press is small (2% scale). The pixels go down when you push them.
//   - `motion-reduce` variants auto-neutralise everything when the user has
//     prefers-reduced-motion set.

// Base transition applied to interactive surfaces.
export const MOTION_BASE =
  'transition-all duration-150 ease-out motion-reduce:transition-none'

// Liftable surface — small hover rise + shadow swell. Use on cards/tiles/buttons.
export const MOTION_LIFT =
  'hover:-translate-y-0.5 hover:shadow-lg motion-reduce:hover:translate-y-0 motion-reduce:hover:shadow-none'

// Pressable — scales down slightly on click. Communicates "I got the click".
export const MOTION_PRESS =
  'active:scale-[0.98] motion-reduce:active:scale-100'

// Combined: the default for buttons and interactive tiles.
export const MOTION_INTERACTIVE = `${MOTION_BASE} ${MOTION_LIFT} ${MOTION_PRESS}`

// Focus ring for keyboard users. Matches the primary colour, offset from the
// surface so it's visible on dark cards.
export const MOTION_FOCUS =
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-background)]'
