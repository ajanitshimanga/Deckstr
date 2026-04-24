# Design Principles

Core philosophy guiding Deckstr development.

## 1. Lightweight First

Minimal resource footprint. Background operation should be nearly invisible.

- Poll sparingly, cache aggressively
- Prefer on-demand over continuous
- Exponential backoff on failures
- Pause when not needed (minimized, game closed)

## 2. Game-Agnostic Architecture

Clean abstractions that work across any game. No game-specific assumptions in core logic.

- Each game integration is an adapter/plugin
- Core handles: accounts, encryption, UI, storage
- Adapters handle: game-specific APIs, rank formats, detection
- Adding a new game should not touch core code

## 3. Extensible Hooks

Easy to add new games and data sources.

**Current:** Riot Games (League, TFT, Valorant)

**Future potential:**
- Steam (CS2, Dota 2)
- Battle.net (Overwatch, WoW)
- Epic Games
- Xbox Live
- PlayStation Network

## 4. Data Users Care About

Surface what matters, skip the noise.

**Priority data:**
- Ranks and divisions
- Win/loss records
- Account identifiers (gamertags, IDs)
- Last played / activity

**Not priority:**
- Match history details
- In-game settings
- Social/friends data

## Implementation Guidelines

### Adding a New Game

1. Create adapter in `internal/<game>client/`
2. Implement detection (process, lockfile, or API)
3. Implement rank fetching
4. Register in game network list
5. No changes to core account/vault logic

### Resource Budget

- Polling: No more frequent than 10s when active
- Background: Prefer 30-60s intervals
- Idle: Zero activity when game not detected
- Memory: Target < 50MB resident

### Async by Default

- Never block the UI thread
- All API calls are async
- Release resources promptly
- Use timeouts on all external calls
