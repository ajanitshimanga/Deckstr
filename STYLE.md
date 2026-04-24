# Deckstr Style Guide

## Table of Contents

- [Design Philosophy](#design-philosophy)
- [Window Sizing](#window-sizing)
- [Color System](#color-system)
- [Component Styles](#component-styles)
  - [Rank Pills](#rank-pills)
  - [Win/Loss Display](#winloss-display)
  - [Account Cards](#account-cards)
- [Layout Principles](#layout-principles)
- [Project Structure](#project-structure)

---

## Design Philosophy

Inspired by Instagram's approach to UI:

1. **Functional Minimalism** - Every element serves a purpose. No decorative clutter.
2. **Whitespace** - Let content breathe. Don't cram elements together.
3. **Limited Color Palette** - Dark background (#0f0f0f), accent colors for meaning only.
4. **Focus on Content** - Accounts are the primary focus. The user should always know what they're looking at.
5. **Full Content Visibility** - Never cut off content. Always ensure all necessary text is visible.

---

## Window Sizing

| State | Size | Min Size | Description |
|-------|------|----------|-------------|
| Login | 380×620 | 380×620 | Vertical, compact for unlock/create vault |
| Main | 520×760 | 520×760 | Wider horizontal for account dashboard |
| Max | 700×900 | - | Maximum allowed size (both states) |

**Dynamic constraints:** The minimum size changes based on state. You cannot resize smaller than the preset for the current state.

The window dynamically resizes:
- On startup → Login size (380×620), min locked to 380×620
- After unlock → Main size (520×760), min locked to 520×760
- On lock → Back to login size, min returns to 380×620

---

## Color System

Defined in `frontend/src/style.css`:

```css
--color-background: #0f0f0f;      /* Deep black */
--color-foreground: #fafafa;      /* Near white */
--color-card: #1a1a1a;            /* Card background */
--color-card-foreground: #fafafa;
--color-primary: #6366f1;         /* Indigo accent */
--color-muted: #262626;           /* Muted backgrounds */
--color-muted-foreground: #a1a1aa;/* Secondary text */
--color-border: #27272a;          /* Subtle borders */
--color-destructive: #ef4444;     /* Red for delete/danger */
--color-success: #22c55e;         /* Green for success */
--color-warning: #f59e0b;         /* Amber for warnings */
```

---

## Component Styles

### Rank Pills

Colored gradient backgrounds based on tier:

| Tier | Gradient | Text Color |
|------|----------|------------|
| Challenger | amber-500/20 → yellow-500/20 | amber-300 |
| Grandmaster | red-500/20 → orange-500/20 | red-400 |
| Master | purple-500/20 → pink-500/20 | purple-400 |
| Diamond | cyan-500/20 → blue-500/20 | cyan-400 |
| Emerald | emerald-500/20 → green-500/20 | emerald-400 |
| Platinum | teal-500/20 → cyan-500/20 | teal-300 |
| Gold | yellow-500/20 → amber-500/20 | yellow-400 |
| Silver | slate-400/20 → gray-400/20 | slate-300 |
| Bronze | orange-600/20 → amber-600/20 | orange-400 |
| Iron | stone-500/20 → gray-500/20 | stone-400 |
| Unranked | zinc-600/20 | zinc-400 |

Style: `px-2.5 py-1 rounded-full text-xs font-semibold border`

### Win/Loss Display

- Wins: `text-green-400 font-medium` (e.g., "12W")
- Losses: `text-red-400 font-medium` (e.g., "8L")
- Winrate badge with conditional color:
  - ≥60%: `text-green-400 bg-green-500/10`
  - ≥50%: `text-emerald-400 bg-emerald-500/10`
  - ≥45%: `text-yellow-400 bg-yellow-500/10`
  - <45%: `text-red-400 bg-red-500/10`

### Account Cards

- Background: `bg-card` with `border-2 border-border`
- Hover: `hover:border-primary/30`
- Active indicator: Left border bar `border-l-4 border-l-primary`
- Avatar: Circle with initials, gradient background
- Display name: `text-base font-semibold`

---

## Layout Principles

1. **Scrollable Filters** - Horizontal scroll for filter chips, no wrapping
2. **Responsive Breakpoints** - xs (480px), sm, md, lg
3. **Mini-client Mode** - Designed to sit in corner without demanding attention
4. **Compact by Default** - Information dense but not cramped

---

## Project Structure

```
Deckstr/
├── main.go                     # Wails entry point, window config
├── app.go                      # App bindings exposed to frontend
├── wails.json                  # Wails configuration
├── STYLE.md                    # This style guide
├── build-installer.bat         # Production build script
│
├── .github/
│   └── workflows/
│       └── release.yml         # Auto-build on tag push
│
├── installer/
│   └── setup.iss               # Inno Setup script
│
├── internal/
│   ├── accounts/
│   │   └── service.go          # Account CRUD operations
│   ├── crypto/
│   │   └── crypto.go           # AES-256-GCM + Argon2id encryption
│   ├── models/
│   │   └── models.go           # Data structures (Account, Rank, etc.)
│   ├── riotapi/
│   │   └── client.go           # Riot Games API client
│   ├── riotclient/
│   │   ├── detection.go        # LCU detection & rank fetching
│   │   └── matching.go         # Account matching logic
│   ├── storage/
│   │   └── storage.go          # Encrypted vault operations
│   └── updater/
│       └── updater.go          # GitHub-based auto-updater
│
├── frontend/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── src/
│   │   ├── main.tsx            # React entry point
│   │   ├── App.tsx             # Root component, routing
│   │   ├── style.css           # Tailwind + custom theme
│   │   ├── lib/
│   │   │   └── types.ts        # TypeScript types
│   │   ├── stores/
│   │   │   └── appStore.ts     # Zustand state management
│   │   └── components/
│   │       ├── ui/             # shadcn/ui base components
│   │       ├── AccountList.tsx # Main account list + cards
│   │       ├── AccountForm.tsx # Add/edit account modal
│   │       ├── LoginScreen.tsx # Unlock/create vault
│   │       └── Settings.tsx    # Settings panel
│   │
│   └── wailsjs/                # Auto-generated Go bindings
│       └── go/
│           ├── main/App.ts     # App method bindings
│           └── models/         # TypeScript model types
│
└── build/
    └── bin/
        └── Deckstr.exe
```

---

## Key Files Reference

| Purpose | File |
|---------|------|
| Window sizing | `main.go:20-27`, `app.go:324-331` |
| Theme colors | `frontend/src/style.css:4-21` |
| Rank tier styles | `frontend/src/components/AccountList.tsx` (getTierStyle) |
| State management | `frontend/src/stores/appStore.ts` |
| Account model | `internal/models/models.go` |
| Encryption | `internal/crypto/crypto.go` |
| Installer script | `installer/setup.iss` |
| Build script | `build-installer.bat` |

---

## Production Build

**Prerequisites:**
- Go 1.21+
- Node.js 18+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Inno Setup 6 (https://jrsoftware.org/isdl.php)

**Build installer:**
```batch
build-installer.bat
```

**Output:** `build\installer\Deckstr-Setup-1.0.0.exe`

**Data locations:**
| Type | Location |
|------|----------|
| Application | `C:\Program Files\Deckstr\` |
| User data | `%APPDATA%\Deckstr\vault.osm` |

---

## Auto-Updates (GitHub Releases)

**How it works:**
1. App checks `https://api.github.com/repos/OWNER/REPO/releases/latest` on startup
2. Compares current version with latest release tag
3. If newer version available, prompts user to update
4. Downloads installer and runs it silently

**Creating a release:**

1. **Update version** in `build-installer.bat`:
   ```batch
   set VERSION=1.1.0
   set GITHUB_OWNER=your-username
   ```

2. **Manual release:**
   ```batch
   build-installer.bat
   ```
   Then upload `build/installer/Deckstr-Setup-1.1.0.exe` to GitHub Releases

3. **Automated release (recommended):**
   ```bash
   git tag v1.1.0
   git push origin v1.1.0
   ```
   GitHub Actions will build and create the release automatically.

**Build flags for version info:**
```
-ldflags "-X 'OpenSmurfManager/internal/updater.Version=1.0.0' -X 'OpenSmurfManager/internal/updater.GitHubOwner=username' -X 'OpenSmurfManager/internal/updater.GitHubRepo=Deckstr'"
```
