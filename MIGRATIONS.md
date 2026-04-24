# Data Migrations & Breaking Changes

This document tracks changes to the vault data format and any migrations required.

## Versioning Strategy

- **Vault Version**: Stored in `vault.version` field (currently: 1)
- **App Version**: Set via git tags at build time (e.g., v1.1.0)

### Rules
1. **Additive changes** (new optional fields): No version bump needed, backwards compatible
2. **Structural changes** (renamed/moved fields): Requires vault version bump + migration code
3. **Removed fields**: Requires deprecation notice first, then vault version bump

---

## Vault Version 1 (Initial Release - v1.0.0)

Initial data structure.

### Breaking Changes
None (initial release)

---

## Changes in v1.1.0 (Current Development)

### Added Fields (Non-Breaking)
- `Vault.passwordHint` (string, optional) - Password hint displayed on lock screen

### New Features
- Password change functionality (re-encrypts vault with new credentials)
- Password hint on account creation
- **Update Password Hint** - Legacy users can add/change hint via Settings menu

### Legacy User Support
Users upgrading from v1.0.0:
- Their vault works immediately (no migration needed)
- Can add a hint anytime via **Settings > Update Password Hint**
- Hint will appear on lock screen after they set one

### Migration Required?
**No** - All changes are additive. Old vaults will work without modification.

---

## Future Breaking Change Template

When making breaking changes:

1. Bump `vaultVersion` constant in `storage.go`
2. Add migration function in `storage.go`:
   ```go
   func (s *StorageService) migrateVaultV1ToV2(vault *models.Vault) error {
       // Migration logic here
   }
   ```
3. Call migration in `loadVaultFile()` based on version check
4. Document the change below

---

## Feature Deprecation Process

When removing a feature:

1. **v X.Y.0**: Mark as deprecated in code comments + UI notice
2. **v X.Y+1.0**: Stop using feature, keep data for rollback
3. **v X.Y+2.0**: Safe to remove data/code

### Currently Deprecated Features
None

### Killed Features
None yet

---

## Rollback Strategy

If a user needs to rollback to a previous version:

1. **Data-compatible versions**: Just install old version
2. **After vault version bump**: Keep backup of pre-migration vault file
   - Location: `%APPDATA%\Deckstr\vault.osm.backup.v{N}`

---

## Testing Migrations

Before releasing a version with migrations:
1. Create test with old vault format
2. Verify migration runs correctly
3. Verify data integrity post-migration
4. Test rollback scenario
