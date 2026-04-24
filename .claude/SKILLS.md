# Deckstr - Development Skills & Context

## Project Overview

Deckstr (formerly OpenSmurfManager) is a secure, local-first account manager for gamers built with:
- **Backend**: Go 1.21+
- **Frontend**: React + TypeScript + Zustand + Tailwind CSS
- **Framework**: Wails v2 (Go + WebView2 desktop apps)
- **Installer**: Inno Setup (Windows)
- **CI/CD**: GitHub Actions

## Project Structure

```
Deckstr/
├── frontend/           # React + TypeScript UI
│   ├── src/
│   │   ├── components/ # UI components (AccountList, UnlockScreen, UpdateModal)
│   │   ├── stores/     # Zustand state management (appStore.ts)
│   │   └── lib/        # Utilities
├── internal/           # Go backend
│   ├── crypto/         # Encryption (AES-256-GCM, Argon2id)
│   ├── storage/        # Vault management
│   ├── models/         # Data structures
│   ├── riotclient/     # LCU integration (detect signed-in account)
│   ├── riotapi/        # Riot API client (ranks, masteries)
│   └── updater/        # Auto-update system (GitHub releases)
├── installer/          # Inno Setup script (setup.iss)
├── build/              # Build assets (icons, manifests)
├── scripts/            # Utility scripts (convert-icon.js)
├── app.go              # Wails bindings (exposed to frontend)
├── main.go             # App entry point
└── .github/workflows/  # CI/CD (release.yml)
```

## User Preferences

### Commit Messages
- Do NOT include "Generated with Claude Code" in commits
- Do NOT include "Co-Authored-By: Claude" in commits
- Keep commit messages clean and focused on changes
- Use conventional commits (feat:, fix:, docs:, etc.)

### Development Approach
- **TDD Preferred**: Write tests FIRST (red), then implement (green)
- **Backwards Compatibility**: Changes should not break existing users
- **Additive Changes**: Prefer adding fields over modifying existing ones
- **Migration Infrastructure**: Use vault versioning for breaking changes

## Key Patterns

### Backwards Compatibility for Vault Data

When adding new fields to the vault:
1. Add field with zero value default (Go handles this automatically)
2. Frontend should handle `undefined` gracefully: `settings?.newField !== false`
3. For breaking changes, increment `Vault.Version` and add migration in `migrateVault()`

Example - checking a new boolean setting that defaults to true:
```typescript
// This handles: true (enabled), undefined (legacy users - treat as enabled), false (disabled)
if (settings?.autoCheckUpdates !== false) {
  checkForUpdates()
}
```

### Adding New Settings

1. Add field to `internal/models/models.go` in `Settings` struct
2. Update `DefaultSettings()` with the default value
3. Add to frontend `appStore.ts` if needed
4. Expose via `app.go` if backend logic needed

### Adding New Backend Methods

1. Implement in appropriate `internal/` package
2. Expose to frontend in `app.go`
3. Run `wails generate module` to update TypeScript bindings
4. Import and use in frontend from `wailsjs/go/main/App`

## Icon Workflow

### Converting SVG to App Icons

Place your SVG at `build/logo.svg`, then run:
```bash
npm run icons
# or
node scripts/convert-icon.js
```

This generates:
- `build/appicon.png` (256x256) - Wails app icon
- `build/windows/icon.ico` - Windows executable icon
- `frontend/src/assets/images/logo-universal.png` (512x512)
- `frontend/src/assets/images/logo.svg`

### Desktop Shortcut Icons
- Inno Setup creates shortcuts with `IconFilename: "{app}\{#MyAppExeName}"`
- If icons appear wrong, user needs to delete shortcut and reinstall
- Windows caches icons; `ie4uinit.exe -show` can help refresh

## Auto-Updater System

### How It Works
1. Version set at build time via ldflags: `-X 'OpenSmurfManager/internal/updater.Version=1.1.0'`
2. On startup, checks `https://api.github.com/repos/{owner}/{repo}/releases/latest`
3. Compares versions; if newer, shows UpdateModal
4. "Update Now" downloads installer to temp, runs with `/SILENT` flag
5. App exits, installer runs, app relaunches automatically

### Update Check Triggers
- On app startup (1.5s delay) - even on lock screen for safety
- Every 30 minutes while app is running
- Manual "Check for Updates" in Settings menu
- Respects `autoCheckUpdates` setting when logged in

### Key Files
- `internal/updater/updater.go` - Backend update logic
- `frontend/src/components/UpdateModal.tsx` - Update notification UI
- `frontend/src/App.tsx` - Update check triggers
- `frontend/src/stores/appStore.ts` - Update state management

## Release Process

**CRITICAL**: Never manually build or upload artifacts. GitHub Actions handles everything.

### Steps to Cut a New Release

1. **Update version** in `frontend/package.json`:
   ```json
   "version": "1.1.X"
   ```

2. **Commit and push**:
   ```bash
   git add -A
   git commit -m "feat: description of changes"
   git push origin master
   ```

3. **Create and push tag**:
   ```bash
   git tag v1.1.X
   git push origin v1.1.X
   ```

4. **Wait for GitHub Actions** - Workflow will:
   - Run Go tests (must pass)
   - Build Windows application with Wails
   - Create Inno Setup installer
   - Create GitHub release with installer attached

### What NOT to Do
- Do NOT run `wails build` locally and upload artifacts
- Do NOT create releases manually with `gh release create`
- Do NOT upload exe files to releases
- Let the workflow handle everything

### If Something Goes Wrong
```bash
# Delete bad release and tag
gh release delete v1.1.X --yes
git push origin :refs/tags/v1.1.X
git tag -d v1.1.X

# After fixes, re-tag
git tag v1.1.X
git push origin v1.1.X
```

### Monitor Release Progress
```bash
gh run watch            # Watch workflow progress
gh release view v1.1.X  # Check release status
```

### CI Pipeline Structure
```
build-windows (Windows) ─┐
                         ├─→ release (needs both to pass)
test (Ubuntu) ───────────┘
```

Tests run: `go test ./internal/... -v` (not `./...` due to embed directive)

## Window Sizing

### Login Window (Compact)
- Width: 380, Height: 700
- Min: 380x650
- Set in `main.go` and `app.go:SetWindowSizeLogin()`

### Main Window (Dashboard)
- Width: 520, Height: 760
- Min: 520x760
- Set in `app.go:SetWindowSizeMain()`

## Testing

### Running Tests
```bash
# All tests
go test ./...

# Specific package with verbose
go test ./internal/storage -v
go test ./internal/crypto -v
```

### Test Files
- `internal/storage/storage_test.go` - Vault operations, password change, hints
- `internal/crypto/crypto_test.go` - Encryption/decryption

## Recovery PIN System (Implemented in v1.1.7+)

### How It Works
- **6-word phrase** generated from BIP39 wordlist (secure entropy)
- Stored as: `RecoveryKeyHash` (Argon2id hash) + encrypted vault key (`RecoveryEscrowedKey`)
- Recovery decrypts escrowed key, re-encrypts vault with new password

### Key Flows
1. **New User**: After account creation → forced PIN reveal modal (shimmer animation)
2. **Legacy User**: On sign-in, if no PIN → forced into same generation flow
3. **Regenerate PIN**: Settings menu → requires password verification
4. **Recovery**: After 3 failed attempts → "Use Recovery PIN" prompt with 6 input cells

### Recovery PIN Files
- `internal/storage/storage.go` - `GenerateRecoveryPhrase()`, `RecoverWithPhrase()`, `RegenerateRecoveryPhrase()`
- `frontend/src/components/RecoveryPhraseModal.tsx` - PIN reveal with shimmer animation
- `frontend/src/components/UnlockScreen.tsx` - Recovery flow, 6-word input with paste support
- `frontend/src/components/AccountList.tsx` - "Regenerate PIN" in settings (with password modal)

### Important Details
- PIN can only be viewed once during generation
- Regenerating invalidates old PIN (new escrowed key)
- Paste support: user can paste "word1 word2 word3..." and it auto-fills all 6 cells

---

## Known Issues / TODO

### [Issue #1] Desktop/Taskbar Icon Shows Wails Logo
**Status**: Open - https://github.com/ajanitshimanga/Deckstr/issues/1

**Problem**: After install/update, desktop shortcut and taskbar show Wails logo, but the actual .exe has correct S logo.

**Tried**:
- Added `IconFilename` to Inno Setup shortcuts
- Verified icon files are correct
- Cleared Windows icon cache

**To Investigate**:
- Copy .ico to install dir and reference explicitly
- Check if Wails build properly embeds appicon.png
- Inno Setup may need icon file copied separately

---

## Common Issues & Fixes

### Shortcut Shows Wrong Icon (Workaround)
- Delete old shortcut, reinstall app
- Or: Right-click → Properties → Change Icon → Browse to exe

### Update Modal Not Showing
- Check version in Settings menu (should not be "dev")
- Ensure release is published (not draft) on GitHub
- Dev builds (`wails dev`) skip update checks

### Window Too Small for Content
- Adjust heights in `main.go` and `app.go:SetWindowSizeLogin()`

### TypeScript Bindings Out of Sync
```bash
wails generate module
```

### Recovery Phrase Invalid Error
- **Cause**: Joining array with empty strings adds extra spaces
- **Fix**: Filter empty strings before join: `words.filter(w => w.trim()).join(' ')`

### Go Tests Fail with "no matching files found"
- **Cause**: `go test ./...` tries to compile main.go which has `//go:embed all:frontend/dist`
- **Fix**: Use `go test ./internal/... -v` to only test internal packages

## Security Model

- **Encryption**: AES-256-GCM with random nonce per save
- **Key Derivation**: Argon2id (64MB memory, OWASP recommended)
- **Storage**: Local only at `%APPDATA%\Deckstr\vault.osm`
- **Plain Text**: Only username and password hint (for login screen)
- **Encrypted**: All account credentials, settings, tags

## Creator Context

The creator is a competitive player who peaked:
- Top 500 in Overwatch
- Top 1000 in Valorant
- Eternity in Marvel Rivals
- Multi-season Grand Champion in Rocket League

The tool exists to play with friends at different skill levels, not to stomp lower ranks. Next goal: Challenger in TFT.

## Useful Commands

```bash
# Development
wails dev                    # Run in dev mode
wails build                  # Build production exe
wails generate module        # Regenerate TS bindings

# Testing
go test ./...                # Run all tests
npm run build                # Build frontend only

# Icons
npm run icons                # Convert SVG to all icon formats

# Git
git tag v1.2.3 && git push origin master && git push origin v1.2.3
```

---

## Pickup Notes (Last Updated: 2026-03-28)

### Current State
- **Version**: v1.2.0 released, working on polling improvements (not yet released)
- **Latest Commit**: `05c2b3c` - fix: polling reliability and profile switching

### What Was Done This Session

**Polling Reliability Fixes:**
1. Profile-aware failure thresholds (instant=1, balanced/eco=2)
2. Fixed infinite backoff bug in catch block (now resets failures after threshold)
3. Profile change restarts polling immediately (applies new interval)
4. HTTP timeout reduced 10s → 2s for faster failure detection
5. HMR duplicate polling fix (clears existing timeout before starting)
6. Reset pollingFailures after clearing account

**New Features:**
- Gameplay detection pauses LCU polling during active games
- Platform-aware process detection (`internal/process/detector.go`)
- `DESIGN_PRINCIPLES.md` added

### To Investigate Next

**Polling Profile Timing Variance:**
- User reported eco mode (15s) felt "too smooth" - profiles might not be applying correctly
- Added debug logging but didn't fully trace the issue
- Debug logs were cleaned up before push

**How to Debug:**
```typescript
// In getPollingIntervals(), add:
console.log('[POLL] Profile:', profile, '| Interval:', intervals.lobbyMs, 'ms')

// In schedulePoll(), add:
console.log('[POLL] Scheduling next poll in', currentBackoff, 'ms')
```

Run `wails dev`, open DevTools (F12), switch profiles in Settings, and watch console to verify intervals change.

### Key Files Changed
- `frontend/src/stores/appStore.ts` - Polling logic, profile switching
- `internal/riotclient/lcu.go` - HTTP timeout (2s)
- `internal/riotclient/detector.go` - Error types
- `internal/process/detector.go` - NEW: Platform process detection
- `internal/models/models.go` - PlatformProcesses, polling constants

### Polling Architecture

```
POLLING_PROFILES (appStore.ts)
├── instant:  { lobbyMs: 1000,  inGameMs: 15000, failureThreshold: 1 }
├── balanced: { lobbyMs: 5000,  inGameMs: 30000, failureThreshold: 2 }
└── eco:      { lobbyMs: 15000, inGameMs: 60000, failureThreshold: 2 }

Flow:
1. getPollingIntervals() reads settings?.pollingProfile
2. pollForActiveAccount() uses intervals for backoff
3. schedulePoll() uses pollingBackoffMs for setTimeout
4. On success: pollingBackoffMs = intervals.lobbyMs
5. On failure: exponential backoff up to 5 min cap
6. After threshold failures: reset to base interval
```

### Not Yet Released
All changes since v1.2.0 are on master but not tagged for release. When ready:
```bash
# Update frontend/package.json version
git tag v1.2.1
git push origin v1.2.1
```
