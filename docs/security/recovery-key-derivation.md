# Recovery key derivation (v2)

This doc describes how Deckstr turns the user's 6-word recovery phrase into
the keys that protect the vault, what v2 fixes relative to v1, and the
migration path existing installs go through.

It's the first section of what will become a broader security architecture
doc. Scope here is deliberately narrow: the recovery-phrase flow only.

## TL;DR

Deckstr's recovery phrase is the user's "I forgot my master password" backup.
It never leaves the device and it's only ever stored in derived form. In v1
(shipped in v1.0–v1.3.0) the derivation had a bug: the verification hash
written to `vault.osm` was byte-identical to the AES key that wrapped the
master vault key, so anyone with read access to the file could decrypt the
vault without ever knowing the phrase. v2 (shipped in v1.3.1) fixes this
with HKDF-based domain separation. Existing vaults are migrated in-place on
the next successful unlock — the old phrase is rotated to a new one and
shown to the user as a mandatory step.

## What the recovery phrase is

A 6-word phrase drawn from an 1101-word list using a cryptographically
secure RNG (`crypto/rand.Int`). Entropy is ~60 bits. This is lower than
BIP-39 on paper but the Argon2id stretching below (64 MB, 3 iterations)
makes an offline brute-force attack impractical: each guess takes ~250ms of
wall-clock time and cannot be meaningfully parallelised by an attacker
without large RAM budgets per core.

The phrase is the user's only fallback if they lose their master password.
It's generated once on vault creation (or on password change) and shown to
the user exactly once via the `RecoveryPhraseModal` — the app never stores
the cleartext.

## Why the v1 scheme was broken

The v1 code did this:

```go
recoveryHash := HashRecoveryPhrase(phrase, salt)   // stored on disk as verification hash
recoveryKey  := HashRecoveryPhrase(phrase, salt)   // used to AES-GCM-encrypt the master vault key
```

`HashRecoveryPhrase` is deterministic Argon2id with identical inputs, so
`recoveryHash` and `recoveryKey` were the same 32 bytes. The hash written
to `vault.osm` **was** the AES-256 key for the wrapped master key.

Attack with read-only access to `vault.osm`:

1. Base64-decode `RecoveryPhraseHash` → 32 bytes
2. Use as AES-256-GCM key with `RecoveryKeyNonce` to decrypt
   `EncryptedVaultKey` → master vault key
3. Use master key with `Nonce` to decrypt `EncryptedData` → plaintext vault

No phrase or password knowledge required.

## The v2 scheme

v2 derives two independent 32-byte keys from the phrase:

```
master     = Argon2id(normalize(phrase), salt, t=3, m=64MB, p=4, 32 bytes)
verifyHash = HKDF-SHA256(master, nil, "deckstr.org/v1/recovery/verify",  32)
encryptKey = HKDF-SHA256(master, nil, "deckstr.org/v1/recovery/encrypt", 32)
```

- `master` is the single expensive step (Argon2id). Rate limits offline
  attackers to ~4 guesses/second.
- `verifyHash` is written to `vault.osm`. Its only use is constant-time
  equality check against a freshly-derived value when the user types their
  phrase. Standing alone it reveals nothing about `encryptKey`.
- `encryptKey` wraps the master vault key via AES-256-GCM. It is
  reconstructed only from the phrase + salt on demand — never persisted.

HKDF guarantees per RFC 5869 §3.3 that different info strings produce
cryptographically independent outputs from the same input key material.
Knowing `verifyHash` doesn't help you derive `encryptKey`.

## Why those exact info labels

The labels follow the convention used in [age's file format
spec](https://github.com/C2SP/C2SP/blob/main/age.md): org-qualified,
slash-separated, version-stamped.

- **`deckstr.org/`** — globally unique prefix. If this code is ever embedded
  in a larger app, the labels cannot collide with another HKDF consumer,
  even accidentally. The domain does not need to actually resolve; it's an
  identifier.
- **`v1/`** — label-scheme version, NOT the vault schema version. Lets us
  evolve labels independently of the on-disk format.
- **`recovery/verify`** vs **`recovery/encrypt`** — slash-separated scope
  for human readability. The two purposes are unambiguously distinct.

Security does not depend on the labels being secret (per RFC 5869, only on
their uniqueness across contexts). They are public in this doc and in the
source.

## Phrase normalization

Both v1 and v2 normalize the phrase before derivation. v1 was:

```go
strings.ToLower(strings.TrimSpace(phrase))
```

This trimmed only leading/trailing whitespace, so `"apple  bear"` (two
spaces) hashed differently than `"apple bear"`. v2 is:

```go
strings.Join(strings.Fields(strings.ToLower(phrase)), " ")
```

This lowercases, splits on any whitespace (collapsing runs), and rejoins
with single spaces. All of `" apple\tbear "`, `"APPLE BEAR"`,
`"apple  bear"` normalize identically. UX bug class closed.

## Vault schema versioning

The `Version` field in `vault.osm` is the **semantics** version — it stamps
the cryptographic scheme, not just the JSON layout.

| Version | Meaning                                                       |
|---------|---------------------------------------------------------------|
| 1       | Broken recovery scheme (verify hash == encrypt key)           |
| 2       | HKDF-derived verify/encrypt keys (current)                    |

`migrateVault` on load inspects the version:
- `== current`: no-op
- `== 1`: sets `needsRecoveryRotation` flag (no disk write yet)
- `> current`: returns `ErrUnsupportedVersion`

The forward-version refusal is a deliberate safety feature: an older build
must not try to open a vault written by a newer build, because its
assumptions about the scheme may be wrong and silent misinterpretation
could corrupt data.

## Migration flow for existing v1 vaults

On the **next successful unlock** of a v1 vault with the correct master
password, the app atomically rewrites the vault to v2 and rotates the
recovery phrase. Specifically:

1. `loadVaultFile` reads `vault.osm` → `Version = 1`.
2. `migrateVault` sets `s.needsRecoveryRotation = true`. No disk write yet.
3. `Unlock` verifies the master password by decrypting `EncryptedData`
   (unchanged from v1). On failure, the flag stays unset and nothing is
   rewritten.
4. On verification success, `rotateRecoveryToV2Locked`:
   a. Generates a fresh recovery phrase.
   b. Derives the new `verifyHash` and `encryptKey` via the v2 scheme.
   c. Wraps the existing master vault key with `encryptKey` (AES-GCM with a
      new nonce).
   d. Re-serializes `vaultData` under the existing master key (new
      nonce — GCM nonces must never repeat under a given key).
   e. Atomically rewrites `vault.osm` with `Version = 2` and the new
      recovery fields.
   f. Parks the cleartext of the new phrase in `pendingRotatedPhrase`.
5. `Unlock` returns success to the caller.
6. The frontend calls `ConsumePendingRecoveryRotation` once. The Go side
   returns the phrase and clears it from memory in the same call so it
   cannot be displayed twice.
7. The frontend forces the `RecoveryPhraseModal` open with rotation copy.
   The user must reveal and acknowledge the new phrase before the app is
   usable. Their old phrase is invalidated.

### Why forced rotation, not opt-in

A v1 vault with `RecoveryPhraseHash` on disk is a timebomb: anyone who ever
gets brief read access to the file (backup location, lost laptop, malware
snapshot) can extract the decrypted credentials indefinitely. The only way
to close that hole is to rotate. Leaving users with a "remind me later"
button would leave the exposure open for an unbounded time.

The UX cost is real — users with their old phrase written on paper have to
re-record a new one — but it's the correct tradeoff for a credential
vault.

### Failure handling

If the atomic rewrite fails mid-flight (disk full, file locked, crash),
the `saveVaultFile` helper uses `os.Rename` on a temp file so the on-disk
vault stays entirely v1 until the new file fully lands. The rotation is
retried on the next successful unlock.

If the user loses the new phrase before recording it, the master password
still works. They can regenerate again via Settings.

## What remains out of scope for v2

This release fixes the derivation bug. It does not close:

- **Memory hygiene of decrypted vault data.** `vaultData` lives on the Go
  heap as strings which are never zeroed; memory dumps or swap files could
  recover the plaintext. Industry-standard desktop password managers have
  the same limitation. Future work: `memguard`-style locked buffers for
  sensitive fields.
- **Integrity of the JSON wrapper.** `EncryptedData` is AES-GCM
  authenticated, but the surrounding JSON fields (Username, PasswordHint,
  recovery fields, timestamps) are not signed. An attacker with write
  access to `vault.osm` could construct a trap vault and social-engineer
  the user. Threat model assumes the user's FS write access is not
  compromised.
- **Formal third-party audit.** The code is open source and this doc
  describes the scheme in full. A professional audit (Cure53, Trail of
  Bits, NCC Group) remains planned for when paying-user count justifies
  the $15–40k cost.

## Invariants enforced by tests

The following properties have dedicated regression tests and would fail
CI if a future change violated them:

- `verifyHash != encryptKey` for fresh v2 vaults
  (`TestRecoveryHashIsNotEncryptionKey` — THE core invariant).
- HKDF info labels domain-separate (`TestDeriveRecoveryKeys_LabelsProvideDomainSeparation`).
- Derivation is deterministic for the same phrase + salt.
- Different salts produce different keys.
- Different phrases produce different keys.
- Whitespace-variant normalization verifies as equal.
- `v1` vaults migrate to `v2` on unlock (`TestUnlockV1VaultMigratesInPlace`).
- Wrong password on v1 vault does NOT migrate (`TestUnlockV1VaultWrongPasswordDoesNotMigrate`).
- `v2` vaults are idempotent on plain unlock.
- `> current` version is refused (`TestUnlockRefusesForwardVersions`).
- Forgot-password path works on v1 vaults and lands on v2.
- `ChangePassword` uses `subtle.ConstantTimeCompare`.

## References

- RFC 5869 — HKDF: https://datatracker.ietf.org/doc/html/rfc5869
- age file format — HKDF label convention:
  https://github.com/C2SP/C2SP/blob/main/age.md
- 1Password two-secret key derivation design:
  https://agilebits.github.io/security-design/deepKeys.html
- Bitwarden security whitepaper (HKDF master-key stretching):
  https://bitwarden.com/help/bitwarden-security-white-paper/
