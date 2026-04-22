// Telemetry wrapper for the frontend. Emits events through the Wails
// LogEvent binding, which forwards them to the Go-side logger.
//
// The typed `track` helpers below act as the **whitelist** of fields that
// ever leave React. Anything not in this file should not be logged — in
// particular: passwords, usernames, Riot IDs, account IDs, display names,
// or tag names. Coarse counts and category IDs only.

import { LogEvent } from '../../wailsjs/go/main/App'

type Level = 'info' | 'warn' | 'error'

// Fire-and-forget. Telemetry failures must never surface to the user.
function emit(level: Level, event: string, attrs: Record<string, string> = {}): void {
  try {
    void LogEvent(level, event, attrs)
  } catch {
    // intentionally empty — no UI impact on telemetry failures
  }
}

export const track = {
  // UI events (the backend already records vault.unlock / account.add /
  // etc. on its own — these are strictly *UI-originated* signals that the
  // Go side can't observe, e.g. which step of the wizard the user stops
  // at, or which filter chip they click).
  wizardStep: (step: 'identity' | 'network' | 'details') =>
    emit('info', 'ui.wizard_step', { step }),

  wizardCancelled: (step: 'identity' | 'network' | 'details') =>
    emit('info', 'ui.wizard_cancel', { step }),

  networkFilterChange: (networkId: string) =>
    emit('info', 'ui.filter_network', { network_id: networkId }),

  gameFilterChange: (gameId: string) =>
    emit('info', 'ui.filter_game', { game_id: gameId }),

  error: (where: string, code: string) =>
    emit('error', 'ui.error', { where, code }),
}
