import { Minus, Square, X, Gamepad2 } from 'lucide-react'
import { cn } from '../lib/utils'
import {
  WindowMinimise,
  WindowToggleMaximise,
  Quit,
} from '../../wailsjs/runtime/runtime'

// Custom window title bar. Replaces the native Windows chrome so the app
// reads as one continuous surface, the way Discord / VS Code / Slack /
// Figma do it. Only visible because main.go sets `Frameless: true`.
//
// Interaction model:
//   - The whole bar carries the `wails-drag` class (CSS var:
//     --wails-draggable: drag). Clicking + dragging anywhere on it moves
//     the window. No-drag children (logo text, buttons) explicitly opt out
//     so they stay clickable.
//   - Window controls call the Wails frontend runtime directly. They don't
//     go through the Go bindings — less round-trip latency for chrome.

export function WindowFrame() {
  return (
    <div
      className={cn(
        'wails-drag',
        'h-9 shrink-0 flex items-center justify-between',
        'bg-[var(--color-background)] border-b border-[var(--color-border)]/40',
        'select-none pl-3',
      )}
    >
      {/* Brand — tiny logo + name. style overrides the drag region so the
          user can select text here (rare, but feels right). */}
      <div className="flex items-center gap-2" style={{ ['--wails-draggable' as any]: 'no-drag' }}>
        <Gamepad2 className="w-3.5 h-3.5 text-[var(--color-primary)]" />
        <span className="text-xs font-semibold text-[var(--color-foreground)] tracking-tight">
          SmurfManager
        </span>
      </div>

      {/* Window controls — no-drag so clicks register. */}
      <div className="flex items-stretch" style={{ ['--wails-draggable' as any]: 'no-drag' }}>
        <ControlButton
          ariaLabel="Minimise"
          onClick={() => WindowMinimise()}
          icon={<Minus className="w-3.5 h-3.5" />}
        />
        <ControlButton
          ariaLabel="Maximise"
          onClick={() => WindowToggleMaximise()}
          icon={<Square className="w-3 h-3" />}
        />
        <ControlButton
          ariaLabel="Close"
          onClick={() => Quit()}
          icon={<X className="w-3.5 h-3.5" />}
          destructive
        />
      </div>
    </div>
  )
}

function ControlButton({
  ariaLabel,
  onClick,
  icon,
  destructive,
}: {
  ariaLabel: string
  onClick: () => void
  icon: React.ReactNode
  destructive?: boolean
}) {
  return (
    <button
      aria-label={ariaLabel}
      onClick={onClick}
      className={cn(
        'w-11 h-9 flex items-center justify-center transition-colors duration-100',
        'text-[var(--color-muted-foreground)]',
        destructive
          ? 'hover:bg-red-500 hover:text-white'
          : 'hover:bg-[var(--color-muted)] hover:text-[var(--color-foreground)]',
      )}
    >
      {icon}
    </button>
  )
}
