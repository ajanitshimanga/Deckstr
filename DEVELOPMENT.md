# Development Guide

This document captures development practices, patterns, and decision-making frameworks for Deckstr.

## Table of Contents
1. [Backwards Compatibility](#backwards-compatibility)
2. [Feature Lifecycle](#feature-lifecycle)
3. [Data Migrations](#data-migrations)
4. [Testing Strategy](#testing-strategy)
5. [Release Process](#release-process)

---

## Backwards Compatibility

### The Golden Rule
**Every new feature should consider users who update from previous versions.**

### Checklist for New Features

Before implementing a feature, ask:

1. **Does it add new data fields?**
   - [ ] New fields should have sensible defaults (empty string, false, nil)
   - [ ] Existing data should remain valid without the new field
   - [ ] Add a way for legacy users to opt-in to the feature

2. **Does it modify existing data structures?**
   - [ ] Requires vault version bump
   - [ ] Requires migration code
   - [ ] Test with data from previous versions

3. **Does it remove functionality?**
   - [ ] Follow deprecation process (see below)
   - [ ] Keep data for at least one version for rollback

4. **Does it change behavior?**
   - [ ] Document the change
   - [ ] Consider a setting to preserve old behavior

### Example: Password Hint Feature

When we added password hints in v1.1.0, we considered:

| Question | Answer |
|----------|--------|
| Does it add new fields? | Yes: `Vault.PasswordHint` |
| Default value? | Empty string (no hint) |
| Can legacy users get it? | Yes: Settings > Update Password Hint |
| Breaking change? | No - JSON unmarshaling ignores missing fields |
| Migration needed? | No - additive change |

**Implementation checklist we followed:**
1. Added `PasswordHint` field to Vault struct (defaults to empty)
2. Added `CreateVaultWithHint()` for new users
3. Added `UpdatePasswordHint()` for legacy users
4. Added UI in settings for all users to manage their hint

---

## Feature Lifecycle

### Stage 1: Experimental
- Feature flag or hidden setting
- May change or be removed
- Not documented publicly

### Stage 2: Beta
- Available to all users
- API may change with notice
- Documented with "beta" label

### Stage 3: Stable
- Full backwards compatibility guaranteed
- Deprecation required before removal
- Breaking changes require major version bump

### Stage 4: Deprecated
```
v X.Y.0   - Mark deprecated, show warning
v X.Y+1.0 - Stop promoting, keep functional
v X.Y+2.0 - Safe to remove
```

### Stage 5: Removed
- Data may be cleaned up
- Document in MIGRATIONS.md

---

## Data Migrations

### When to Bump Vault Version

| Change Type | Version Bump? | Migration Code? |
|-------------|---------------|-----------------|
| Add optional field | No | No |
| Add required field with default | No | No |
| Rename field | Yes | Yes |
| Change field type | Yes | Yes |
| Remove field | Yes | Maybe |
| Restructure data | Yes | Yes |

### Migration Code Pattern

```go
// In storage.go
func (s *StorageService) migrateVault(vault *models.Vault) error {
    // Migration from v1 to v2
    if vault.Version < 2 {
        // Perform migration
        vault.NewField = computeDefault(vault)
        vault.Version = 2
    }

    // Migration from v2 to v3
    if vault.Version < 3 {
        // Next migration...
        vault.Version = 3
    }

    return nil
}
```

### Testing Migrations

1. Create test fixtures with old data format
2. Run migration
3. Verify data integrity
4. Test functionality with migrated data
5. Test rollback scenario

---

## Testing Strategy

### Red-Green-Refactor (TDD)

1. **Red**: Write failing test first
   ```bash
   go test ./internal/storage -run TestNewFeature
   # Should fail: method doesn't exist
   ```

2. **Green**: Implement minimum code to pass
   ```bash
   go test ./internal/storage -run TestNewFeature
   # Should pass
   ```

3. **Refactor**: Clean up while keeping tests green

### Test Categories

| Category | Purpose | Location |
|----------|---------|----------|
| Unit | Test individual functions | `*_test.go` alongside code |
| Integration | Test component interaction | `internal/*/` |
| Migration | Test data upgrades | `storage_test.go` |

### Coverage Goals

- Crypto operations: 100%
- Storage operations: 90%+
- Business logic: 80%+
- UI: Manual testing

---

## Release Process

### Semantic Versioning

```
MAJOR.MINOR.PATCH
  │     │     └── Bug fixes, no new features
  │     └──────── New features, backwards compatible
  └────────────── Breaking changes
```

### Pre-release Checklist

1. [ ] All tests pass: `go test ./...`
2. [ ] Frontend builds: `cd frontend && npm run build`
3. [ ] MIGRATIONS.md updated
4. [ ] No TODO/FIXME in committed code
5. [ ] Version consistency (package.json matches tag)

### Release Steps

```bash
# 1. Ensure everything is committed
git status

# 2. Create and push tag
git tag v1.1.0
git push origin v1.1.0

# 3. GitHub Actions builds and creates release
```

### Post-release

1. Monitor GitHub Issues for reports
2. Keep previous version available for rollback
3. Update documentation if needed

---

## Quick Reference

### Adding a New User-Facing Feature

```
1. Ask: Can legacy users access this?
2. Write tests first (TDD)
3. Implement with defaults for existing data
4. Add UI for legacy users to opt-in
5. Update MIGRATIONS.md
6. Update this guide if pattern is reusable
```

### Removing a Feature

```
1. Mark deprecated (add warning)
2. Wait one minor version
3. Stop promoting (hide from new users)
4. Wait one more minor version
5. Remove code and data
6. Document in MIGRATIONS.md
```

### Emergency Rollback

If users report critical issues after update:
1. Users can reinstall previous version
2. Vault data is backwards compatible (we don't bump version for additive changes)
3. If vault version was bumped, backup exists at `vault.osm.backup.v{N}`
