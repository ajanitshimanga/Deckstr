# Contributing to Deckstr

Thanks for your interest in contributing! This guide will help you understand the codebase, contribution process, and review requirements.

## Table of Contents
- [Architecture Overview](#architecture-overview)
- [Before You Start](#before-you-start)
- [Contribution Workflow](#contribution-workflow)
- [Code Review Process](#code-review-process)
- [Adding New Platforms](#adding-new-platforms)
- [Code Standards](#code-standards)

---

## Architecture Overview

```
Deckstr/
├── internal/
│   ├── models/        # Data structures (GENERIC - works for any platform)
│   ├── crypto/        # AES-256-GCM encryption (GENERIC - don't modify)
│   ├── storage/       # Vault persistence (GENERIC)
│   ├── accounts/      # CRUD operations (GENERIC)
│   ├── providers/     # Provider interface + registry (GENERIC)
│   │   ├── contract/  # Reusable conformance test suite
│   │   ├── fake/      # In-memory fake for tests
│   │   └── riot/      # Riot adapter (PLATFORM-SPECIFIC)
│   ├── riotclient/    # Riot LCU protocol (RIOT-SPECIFIC)
│   ├── riotapi/       # Riot public API (RIOT-SPECIFIC)
│   └── process/       # OS process detection (GENERIC)
├── test/e2e/          # Service integration tests (uses fakes)
├── frontend/src/      # React + TypeScript UI
└── app.go             # Wails bindings (API surface)
```

### Key Architectural Points

| Layer | Status | Notes |
|-------|--------|-------|
| Encryption | Generic | AES-256-GCM, Argon2id key derivation. **Do not modify.** |
| Data Model | Multi-platform ready | `Account.NetworkID` supports multiple platforms |
| Process Detection | Extensible | Add processes to `DefaultGameNetworks()` |
| Account Detection | Provider-based | Implement `providers.Provider`; registered in `app.go` |

---

## Before You Start

### 1. Check for Existing Work

Before starting a new feature:
- Check [open issues](../../issues) for related discussions
- Check [open PRs](../../pulls) for work in progress
- For new platforms (Epic, Steam, etc.), open an issue first to discuss approach

### 2. Understand the Scope

**Changes that can be merged quickly:**
- Bug fixes with tests
- UI improvements
- Documentation
- Adding games to existing platforms (new Riot games, etc.)
- Performance improvements

**Changes that require discussion first:**
- New platform integrations (Epic, Steam, Battle.net)
- Architectural changes
- New encryption schemes
- Changes to the vault format

### 3. Development Setup

```bash
# Prerequisites
# - Go 1.21+
# - Node.js 18+
# - Wails CLI (go install github.com/wailsapp/wails/v2/cmd/wails@latest)

# Clone and run
git clone https://github.com/yourusername/Deckstr.git
cd Deckstr
wails dev

# Run tests
go test ./...
```

---

## Contribution Workflow

### Step 1: Open an Issue

For anything beyond trivial fixes, open an issue first:

```markdown
## What I Want to Do
[Brief description]

## Proposed Approach
[How you plan to implement it]

## Questions/Concerns
[Any architectural decisions you need input on]
```

### Step 2: Fork and Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/bug-description
```

**Branch naming:**
- `feature/` - New features
- `fix/` - Bug fixes
- `refactor/` - Code improvements without behavior changes
- `docs/` - Documentation only

### Step 3: Make Changes

- Write tests for new functionality
- Follow existing code patterns
- Keep commits focused and atomic
- Write clear commit messages

### Step 4: Submit PR

Use this template:

```markdown
## Summary
[1-3 bullet points describing the change]

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Refactor
- [ ] Documentation

## Testing Done
- [ ] Added/updated unit tests
- [ ] Manual testing performed
- [ ] Tested on Windows
- [ ] Tested on macOS (if applicable)

## Breaking Changes
[None / describe any breaking changes]

## Screenshots (if UI changes)
[Before/after if applicable]
```

---

## Code Review Process

### Review Criteria

All PRs are evaluated on:

1. **Correctness** - Does it work? Are edge cases handled?
2. **Security** - No credential leaks, injection vulnerabilities, or weakened encryption
3. **Maintainability** - Is the code readable and well-structured?
4. **Scope** - Does it do one thing well? Avoid scope creep.
5. **Tests** - Is the change tested?

### Review Timeline

- **Small fixes (<50 lines):** 1-3 days
- **Medium features:** 3-7 days
- **Large changes/new platforms:** May require multiple review rounds

### What Will Block a PR

- No tests for new functionality
- Modifying encryption without security review
- Breaking changes without migration path
- Large PRs that should be split up
- Code that doesn't follow existing patterns

### Merge Requirements

- At least 1 maintainer approval
- All CI checks passing
- No unresolved review comments
- Rebased on latest `master`

---

## Adding New Platforms

The codebase has a `providers.Provider` interface that all platform integrations must implement. Adding a new platform (Epic, Steam, etc.) is a drop-in operation - no changes to `app.go` or core services required.

### What You DON'T Need to Touch

- `internal/crypto/` - Encryption is generic AES-256-GCM
- `internal/storage/` - Vault storage works for any platform
- `internal/models/` - Account model already has `NetworkID` field
- `internal/providers/registry.go` - Dispatch is automatic

### What You WILL Create

For a new platform (e.g., Epic Games):

```
internal/providers/
└── epic/
    ├── provider.go        # Implements providers.Provider
    └── provider_test.go   # Calls contract.RunContractTests
```

Then register it in `app.go` startup with one line:

```go
a.providers.MustRegister(epic.New())
```

### The Provider Contract

Your provider must satisfy `providers.Provider` (see `internal/providers/provider.go`):

```go
type Provider interface {
    NetworkID() string                                              // "epic"
    DisplayName() string                                            // "Epic Games"
    IsClientRunning(ctx context.Context) bool
    Detect(ctx context.Context) (*DetectedAccount, error)
    MatchAccount(accounts []models.Account, detected *DetectedAccount) *models.Account
    UpdateAccount(account *models.Account, detected *DetectedAccount)
}
```

### Required: Pass the Contract Suite

Every provider MUST pass the conformance suite. Add this test:

```go
// internal/providers/epic/provider_test.go
package epic_test

import (
    "testing"
    "OpenSmurfManager/internal/providers"
    "OpenSmurfManager/internal/providers/contract"
    "OpenSmurfManager/internal/providers/epic"
)

func TestEpicProvider_Contract(t *testing.T) {
    contract.RunContractTests(t, contract.Suite{
        Factory: func() providers.Provider { return epic.New() },
    })
}
```

If this passes, the basics are correct. Reviewers will check it first.

### Recommended Approach

1. **Open an issue** describing the platform, detection mechanism, and any security/legal concerns (reverse-engineered protocols, EULA constraints, etc.)
2. **Implement the Provider interface** - look at `internal/providers/riot/` as a reference adapter
3. **Pass the contract suite** before opening a PR
4. **Add an integration test** in `test/e2e/` if your provider has unusual matching logic

### Reference Implementation

`internal/providers/riot/provider.go` shows the adapter pattern: keep platform-specific protocol code in its own package (e.g. `internal/riotclient/`) and have the provider package translate between native types and the generic `DetectedAccount`.

---

## Code Standards

### Go Code

```go
// Good: Clear function names, error handling
func (s *Service) GetAccount(id string) (*Account, error) {
    if id == "" {
        return nil, ErrInvalidID
    }
    // ...
}

// Avoid: Unclear names, panics
func (s *Service) Get(i string) *Account {
    // Don't panic on errors
}
```

- Use `context.Context` for cancellation
- Return errors, don't panic
- Use meaningful variable names
- Keep functions focused (<50 lines preferred)

### TypeScript/React

```typescript
// Good: Typed props, clear component names
interface AccountCardProps {
  account: Account;
  onSelect: (id: string) => void;
}

export function AccountCard({ account, onSelect }: AccountCardProps) {
  // ...
}
```

- Use TypeScript strictly (no `any` without justification)
- Prefer functional components with hooks
- Keep components focused

### Commit Messages

```
feat: add Epic Games account detection
fix: handle missing lockfile gracefully
refactor: extract provider interface from riotclient
docs: add contribution guidelines
test: add unit tests for account matching
```

Prefixes: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`

### Security

**Never commit:**
- API keys or secrets
- Personal account data
- Hardcoded credentials

**Always:**
- Use the existing crypto service for sensitive data
- Validate user input
- Handle errors gracefully (don't leak stack traces)

---

## Questions?

- Open an issue for technical questions
- Check existing issues/discussions first
- For security concerns, email directly (don't open public issues)

Thanks for contributing!
