# Telemetry

This document describes **exactly** what usage data Deckstr can
collect when you opt in, where it goes, and how to control it. If the
code ever diverges from this document, the document is the source of
truth — please open an issue.

## Two tiers, two guarantees

| Tier | What | Guarantee |
|------|------|-----------|
| **Vault data** | Accounts, passwords, usernames, Riot IDs, notes, tags, display names | **Never transmitted. Ever.** Encrypted on disk, zero-knowledge: we cannot access it. |
| **Usage analytics** | Coarse event counts and timings (see below) | **Opt-in, off by default.** Local-only today. If/when remote shipping lands, re-prompted separately and you can still decline. |

Vault data and usage data live in **separate code paths**. The telemetry
package has an explicit attribute whitelist enforced by tests — by
construction it cannot emit vault fields.

## Default state

- **Off.** Out of the box, no events are written, no client identifier is
  created, no file in the logs directory is touched. If you never opt in,
  there is no telemetry record at all.
- **On when you opt in.** A one-line prompt on first unlock asks whether
  you want to help. You can change the setting any time via
  Settings → Usage Analytics.

## What we collect (when you opt in)

All events share a common "resource" block and a per-event "attributes"
block. The format mirrors the OpenTelemetry log record schema so the
data is portable to any standard backend.

### Resource (sent with every event)

| Field | Example | Purpose |
|-------|---------|---------|
| `service.name` | `"Deckstr"` | Distinguishes our events from other OTel sources |
| `service.version` | `"1.2.5"` | Lets us correlate regressions with releases |
| `os.type` | `"windows"` | Platform-level bucketing (Windows / macOS / Linux) |
| `$session_id` | UUID, new each launch | Groups events from a single app session. Named with a `$` prefix so PostHog's session-grouping UI picks it up natively. |
| `client.id` | **rotating daily** salted hash, *or omitted entirely* | Enables daily/monthly active-user counts without a durable tracking identifier. See "Client identifier" below. |

### Events

| Event | Attributes | What it tells us |
|-------|-----------|------------------|
| `app.start` | `startup_latency_ms`, `has_vault` (bool) | Cold-boot performance; new-install vs returning |
| `vault.unlock` | `success` (bool) | Unlock success rate (catches regressions in login flow) |
| `vault.lock` | — | Lock action counts |
| `account.add` | `network_id` (e.g. `"riot"`), `games_count`, `tags_count`, `success` | Which networks get used; rough usage shape |
| `account.edit` | `network_id`, `success` | Edit usage + error rate |
| `account.delete` | `success` | Delete usage + error rate |
| `account.detect` | `result` (`none`/`detected`/`error`), `duration_ms` | Detection reliability |
| `ui.wizard_step` | `step` (`identity`/`network`/`details`) | Wizard funnel; where users drop off |
| `ui.wizard_cancel` | `step` | Abandonment point |
| `ui.filter_*` | `network_id` / `game_id` | Which filters are useful |
| `ui.error` | `where`, `code` | Frontend errors we'd otherwise never see |

## What we **never** collect

Explicit deny-list, enforced by the attribute whitelist in
`internal/telemetry` and checked by a Go test against the instrumented
call sites:

- Passwords (master or per-account)
- Usernames (the ones inside your vault)
- Riot IDs
- Account IDs or PUUIDs
- Display names
- Tag names (user-created strings)
- Notes
- Anything else stored in the vault

Metadata you'd reasonably consider identifying — IP addresses, device
names, hostnames, MAC addresses — is also never collected.

## Client identifier

DAU / MAU requires some way to count unique users. We use the **least
identifying** approach that still answers the question:

- A salted hash rotated on a daily boundary. The salt changes each
  calendar day, so two events from the same user on different days
  produce different hashes. Daily active users is computable
  (unique hashes per day); 28-day rolling MAU is computable
  (union of 28 days' sets); cross-day retention and cohorts
  intentionally are not.
- When the setting is off, the hash is never computed and the salt
  file is never created.

This is a deliberate tradeoff — weaker analytics, stronger privacy
posture. If we ever need stable long-term IDs (for cohort analysis),
that will be a separate, explicit opt-in with its own prompt.

## Where the data goes

- **Today:** `%APPDATA%\Deckstr\logs\app.log` (Windows; legacy `OpenSmurfManager\` installs are auto-migrated on first launch) and
  the rotated backups `app.log.1`, `app.log.2`, `app.log.3`. Total disk
  usage is capped at ~4 MB per install. Nothing is transmitted off
  your machine.
- **Future (not yet shipped):** an opt-in HTTP shipper will POST these
  events to a collector we host. When that ships, the app will prompt
  separately — agreeing to local logs does **not** roll into agreeing
  to remote shipping.

## Controls you have

- **Turn it off** — Settings → Usage Analytics → Off. Stops writing
  future events immediately. Existing logs are not touched until you
  explicitly delete them.
- **View your logs** — Settings → Usage Analytics → Open logs folder.
  Opens the directory in your file manager so you can read the raw
  JSON-line records.
- **Export your logs** — same folder; just copy the files.
- **Delete your logs** — Settings → Usage Analytics → Delete all. Wipes
  the logs directory and the client-ID salt file. If you opt back in
  later, a fresh salt is generated.

## How we enforce this

- The attribute whitelist lives in
  `frontend/src/lib/telemetry.ts` (for UI events) and in the call
  sites inside `app.go` (for backend events). Reviewers check PRs that
  touch these files with extra care.
- A Go test greps the telemetry call sites for banned field names
  (`password`, `username`, `riotId`, `puuid`, `displayName`, `notes`,
  `tags`) and fails the build if any appear.
- Every event in this document must have a corresponding call site;
  every call site must have a corresponding row. PRs that add an
  event update this file in the same commit.

## Reporting telemetry concerns

If you believe the app is collecting something it shouldn't, please
open a GitHub issue (or email privately for sensitive reports). We take
the gap between this document and the code very seriously — it's the
only guarantee that matters.
