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

type WizardStep = 'identity' | 'game' | 'network' | 'details'

export const track = {
  // UI events (the backend already records vault.unlock / account.add /
  // etc. on its own — these are strictly *UI-originated* signals that the
  // Go side can't observe, e.g. which step of the wizard the user stops
  // at, or which filter chip they click).
  wizardStart: () =>
    emit('info', 'ui.wizard_start', {}),

  wizardStep: (step: WizardStep) =>
    emit('info', 'ui.wizard_step', { step }),

  wizardGameSelect: (gameId: string) =>
    emit('info', 'ui.wizard_game_select', { game_id: gameId }),

  wizardNetworkSelect: (networkId: string) =>
    emit('info', 'ui.wizard_network_select', { network_id: networkId }),

  wizardCancelled: (step: WizardStep) =>
    emit('info', 'ui.wizard_cancel', { step }),

  wizardSubmit: (gameId: string, networkId: string) =>
    emit('info', 'ui.wizard_submit', { game_id: gameId, network_id: networkId }),

  networkFilterChange: (networkId: string) =>
    emit('info', 'ui.filter_network', { network_id: networkId }),

  gameFilterChange: (gameId: string) =>
    emit('info', 'ui.filter_game', { game_id: gameId }),

  error: (where: string, code: string) =>
    emit('error', 'ui.error', { where, code }),
}
