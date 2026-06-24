# Maintainer Identity

## Findings

The duplicate maintainer records were mostly not created by the `/lfx` run.

From the local DB behind `reports/duplicate-github-maintainers.csv`:

- `59` normalized GitHub handles are duplicated.
- Those correspond to `122` maintainer records, so `63` records are extra beyond one record per handle.
- Creation dates:
  - `115` records created on `2026-01-08`
  - `1` on `2026-01-14`
  - `3` on `2026-01-15`
  - `2` on `2026-01-29`
  - `1` on `2026-04-13`

The LFX/foundation observations in the CSV are mostly from `2026-06-15` and `2026-06-16`, but they are observations attached to already-existing maintainer rows. They are not evidence that `/lfx` created the duplicate maintainer records.

## Likely Cause

The initial importer in `db/bootstrap.go` creates maintainers keyed by email:

```go
tx.Where("email = ?", email).FirstOrCreate(&maintainer)
```

That explains the CSV pattern: the same GitHub handle appears with different emails across projects, so the importer treated them as separate people.

Examples from the report:

- `amisevsk`: Red Hat email and Jozu email
- `bmicklea`: Jozu email and Gmail
- `blixtra`: Microsoft email and Gmail
- `bacongobbler`: Fermyon email and Fishworks email
- `bergwolf`: same email, separate rows across projects

There are also suspicious rows where the same GitHub value appears to be an org/shared handle rather than a person, for example `BoCloud` on two different FabEdge maintainers. Those need cleanup before enforcing uniqueness.

`db/bootstrap.go` is no longer in use; the background is as follows: historically the CNCF operated an internal worksheet that recorded the maintainer rosters for each CNCF Project.

The worksheet became unworkable over time as the number of CNCF Project Maintainers increased over the years. The reason it existed as an internal CNCF file was that it held the email addresses that maintainers shared with the CNCF.

The CNCF needed to know maintainer email addresses but it never published the list of email addresses of CNCF Project Maintainers as a block.

maintainer-d as a database application initially read from the internal worksheet, and `db/bootstrap.go` was the code that read that file to populate data in the maintainer-d database. Bootstrap was used when maintainer-d was initially deployed to production, but it is no longer used, favoring instead the data in maintainer-d.


## Duplicate Categories

The duplicate groups in `reports/duplicate-github-maintainers.csv` are **not all the
same kind of problem**. Cleanup logic must classify each group before acting on it.
Four categories are visible in the current data:

**SAFE_MERGE — same person, corroborated.** Names match (case-insensitive) and either
the LFX user ID matches across records or one record is simply unmatched. Examples:

- `bergwolf` (479, 1969): same LFX ID `bergwolf`; the name is a word-order variant of
  the same person.
- `rickbrouwer` (2090, 2091): identical across all identity fields — a pure duplicate
  row.
- `rchincha`, `rchamarthy`, `hallyn`, `gavrissh`, `jiangphcn`: one record LFX-matched,
  the other not, but the handle and person clearly agree.

**REVIEW_REQUIRED — different people, or a shared/org handle.** Must never be
auto-merged. Examples:

- `bocloud` (1292, 1293): two different people in the same org sharing an org account
  on FabEdge. Names omitted.
- `feynmanzhou` (307, **1203**, 1204): id 1203 is a different person entirely.
  Names omitted.

**CORRUPT_DATA — non-identity text in an identity field.** Needs investigation before
any matching or merging. Examples:

- `salaboy` (1266): the email field holds freeform text, not an address — [redacted].
- `kingdonb` (540): the name and the email/LFX match point to two different people.
  Names omitted.

**Near-duplicate typo emails.** Likely a typo rather than a real second address; flag,
do not silently merge. Example: `hiddeco` (530, 1550) — two near-identical emails
differing by one trailing character (likely a typo). Emails redacted.

Note: `robertkielty` (1941, 2060) is the operator's own record (two emails on
maintainerd). It is a good known-good first target for testing the merge script.


## Model Constraint

Do not use a plain composite unique key like `(normalized_github, deleted_at)`. Because `deleted_at` is nullable, Postgres would still allow duplicates where `deleted_at IS NULL`.

Use a partial unique index instead:

```sql
create unique index concurrently idx_maintainers_unique_normalized_github_active
on maintainers (lower(btrim(git_hub_account)))
where deleted_at is null
  and nullif(btrim(git_hub_account), '') is not null
  and lower(btrim(git_hub_account)) <> 'github_missing';
```

That would disallow two active maintainer records with the same normalized GitHub handle, while still allowing soft-deleted historical rows and sentinel/missing handles.

If `normalized_github` should be explicit in `model/main.go`, prefer a generated column plus a SQL migration:

```sql
alter table maintainers
add column normalized_github text
generated always as (lower(nullif(btrim(git_hub_account), ''))) stored;

create unique index concurrently idx_maintainers_unique_normalized_github_active
on maintainers (normalized_github)
where deleted_at is null
  and normalized_github is not null
  and normalized_github <> 'github_missing';
```

Then `model.Maintainer` could expose it read-only:

```go
NormalizedGitHub string `gorm:"column:normalized_github;->"`
```

Do not rely on GORM tags alone for this. For a production data-integrity rule, use an explicit Postgres migration.

## Data Model Gap

The root model issue is email identity. A maintainer can have multiple emails:

- manually submitted emails to the CNCF sent to Project Team email addresses
- GitHub commit/profile email
- LFX email
- employer email over time which are unreliably recorded in GITDM

The better model is:

- `maintainers`: one person record
- `maintainer_emails`: many emails per maintainer, with `source`, `is_primary`, `verified_at`, timestamps
- `maintainer_identities`: external identities, e.g. GitHub login/user ID, LFX LFID/user ID

Then enforce uniqueness on identity, not on email:

- active GitHub identity normalized login unique
- active LFX identity unique
- email uniqueness more carefully, because emails can move, be aliases, or be stale

## Recommended Order

1. Clean/merge current duplicate maintainer records.
2. Fix import/upsert paths to match by normalized GitHub before email.
3. Stop matching on sentinel email values like `EMAIL_MISSING`.
4. Add the Postgres partial unique index.
5. Later, split emails/identities into proper child tables.

## Implementation Instructions

Use these prompts as staged implementation tasks for a coding agent. Keep each change small enough to review independently, and do not add SQLite-specific behavior for this work.

### Step 1: Clean And Merge Current Duplicate Maintainer Records

Prompt:

```text
Audit and clean duplicate maintainer records that share the same normalized GitHub handle.

Use reports/duplicate-github-maintainers.csv and scripts/duplicate-github-maintainers-local.sh as the starting point. Add a Postgres-only SQL review script under scripts/ or reports/ that identifies duplicate active maintainer rows by lower(btrim(git_hub_account)), excluding blank and GITHUB_MISSING handles.

Before producing any merge candidates, classify each duplicate group. The real data is mixed and a single "pick a canonical record" rule is unsafe:
- SAFE_MERGE: names match case-insensitively AND the LFX user ID matches across records (or one record is simply unmatched). Same real person.
- REVIEW_REQUIRED: names differ, OR the handle looks like an org/shared account, OR LFX observations link to different real people. Examples in the current data: bocloud (1292, 1293 — two different people, names omitted); feynmanzhou (1203 is a different person entirely, names omitted).
- CORRUPT_DATA: an email or name field contains non-identity text that is not a recognized sentinel (e.g. salaboy id 1266 email holds freeform text [redacted]; kingdonb id 540 whose email/LFX point to a different person, names omitted).
- NEAR_DUPLICATE_EMAIL: emails differ by an obvious typo (e.g. hiddeco hhh.computers vs hhh.computer) — flag, do not auto-merge.

Only produce merge candidates for SAFE_MERGE groups. Emit REVIEW_REQUIRED, CORRUPT_DATA and NEAR_DUPLICATE_EMAIL groups as separate sections with no data changes proposed and a one-line reason each.

For a SAFE_MERGE group, choose the canonical record with this deterministic tiebreaker: prefer maintainer_status = Active, then an LFX-matched record, then the lowest id as a stable fallback.

There is no existing maintainer merge/dedup or soft-delete helper in the repo. The only precedent is MergeCompanies (db/store_impl.go:488), which reassigns foreign keys then deletes the source row; model a maintainer merge on that reassign-then-(soft-)delete shape. Note that no code currently uses gorm Unscoped, so soft-delete via DeletedAt is the conservative default.

Produce a dry-run merge script that, for each SAFE_MERGE group, reports what would be moved from the duplicate(s) onto the canonical record:
- maintainer_projects rows
- maintainer_identity_observations rows
- audit_logs maintainer_id references
- remote_team_users rows
- any other foreign-key-like references to maintainers

The script must be safe by default:
- dry-run is the default
- the apply path must be explicit
- every selected canonical record and deleted/soft-deleted duplicate must be listed in output
- do not hard-delete maintainers unless the existing repository pattern clearly supports it; prefer soft-delete or a conservative no-delete first pass

Before changing data, add queries that show:
- duplicate group
- canonical maintainer candidate
- project memberships before/after
- LFX and foundation observations before/after
- audit log references before/after

Do not apply production data changes automatically from application startup. This should be an operator-run Postgres script.
```

Acceptance checks:

- The report shows zero duplicate active maintainers for normalized GitHub handles **among SAFE_MERGE groups** after the apply script is run on a local production snapshot.
- Groups bocloud and feynmanzhou appear in REVIEW_REQUIRED output with no merge proposed.
- Groups salaboy and kingdonb appear in CORRUPT_DATA output with no merge proposed.
- Every REVIEW_REQUIRED / CORRUPT_DATA / NEAR_DUPLICATE_EMAIL group remains unmerged after apply, and the script lists exactly which groups were skipped and why.
- No project membership is lost.
- No maintainer identity observation is orphaned.
- No audit log reference is orphaned.
- The script can be run repeatedly without creating additional changes.

### Step 2: Match By Normalized GitHub Before Email

Prompt:

```text
Update maintainer creation and linking paths so identity matching prefers normalized GitHub handle before email.

Current state to build on, not redo:
- db/store_impl.go UpsertMaintainerWithIdentity (around lines 217-234) ALREADY matches by lower(git_hub_account) first and only falls back to email when no GitHub match is found. Treat this as the reference behavior; the work is to make the other paths consistent with it, not to reimplement it.

Remaining paths to fix:
- db/bootstrap.go maintainer import path uses tx.Where("email = ?", email).FirstOrCreate(&maintainer). This importer is no longer used, so remove it from the repo as part of this work. In its place (and in a comment near UpsertMaintainerWithIdentity) leave a short breadcrumb: relying on email as a unique identifier was the original mistake; identity matching must prefer the GitHub handle.
- any web-bff maintainer creation or "from ref" flow that independently searches by email/GitHub.

Do not write a new normalization helper. Reuse the existing NormalizeGitHubHandle (dotproject/maintainers_patch.go:126), which lower-cases and trims whitespace and surrounding @ " ' characters. If a path cannot import that package cleanly, factor the existing function into a shared location rather than duplicating lower/trim logic (the repo currently has several ad-hoc copies).

Required matching behavior:
- If a usable GitHub handle is present, look up an existing maintainer by lower(btrim(git_hub_account)) first.
- If a maintainer is found by GitHub, update/link that maintainer instead of creating a new row.
- Only fall back to email matching if no usable GitHub handle is present or no GitHub match exists.
- Email fallback must use only usable email addresses; sentinel values such as email_missing are not usable.
- Do not overwrite a good existing email with EMAIL_MISSING or another sentinel.
- Do not overwrite a good existing GitHub handle with GITHUB_MISSING.

Add focused tests showing:
- same GitHub + different email links to the existing maintainer
- same email + missing GitHub still links by email
- different GitHub + same email does not accidentally merge two distinct identities without an explicit decision
- project association is added once and remains idempotent
```

Acceptance checks:

- `go test ./db ./dotproject ./cmd/web-bff` passes.
- Existing import/upsert tests cover the GitHub-first path.
- The duplicate report does not grow after re-running import/sync locally.

### Step 3: Stop Matching On Sentinel Email Values

Prompt:

```text
Prevent sentinel values from participating in identity matching.

Audit all maintainer lookup paths that match by email or GitHub. Treat these values as missing identity data:
- ""
- EMAIL_MISSING
- GITHUB_MISSING
- GITHUB_EMAIL_MISSING
- whitespace-only values

At minimum, update db/store_impl.go UpsertMaintainerWithIdentity so it never runs the email fallback query with EMAIL_MISSING.

Add small helper functions if useful:
- usableEmail(value string) bool
- usableGitHub(value string) bool

usableEmail must be a STRUCTURAL check, not just a sentinel-string blocklist. The real data contains freeform garbage in the email column (e.g. salaboy id 1266 email "can def say somethintg to him about it"), which is not a sentinel but must not participate in matching. At minimum require a single "@" with non-empty local and domain parts after trimming; reject anything else.

These sentinels are currently duplicated string literals with inconsistent casing across db/, cmd/web-bff/, lfx/ and dotproject/ (there is no shared constant). Prefer introducing one shared, case-insensitive set of sentinel constants and using it in the helpers. While here, note the likely bug at model/main.go:92 where Maintainer.GitHubEmail defaults to GITHUB_MISSING instead of GITHUB_EMAIL_MISSING; do not silently "fix" it without confirming, but the helpers should treat both as missing.

Keep the helper semantics local and obvious. Do not introduce broad abstractions unless needed by multiple paths.

Tests must prove:
- UpsertMaintainerWithIdentity with email EMAIL_MISSING does not match an unrelated existing maintainer that also has EMAIL_MISSING.
- A structurally invalid email (no "@", or freeform text) is treated as missing and does not match.
- A missing/sentinel GitHub handle does not match GITHUB_MISSING records.
- Existing behavior for real email and real GitHub values remains intact.
```

Acceptance checks:

- The specific bad-link class is covered by a regression test.
- No query path uses sentinel identity values to select an existing maintainer.
- Existing sentinel storage remains allowed for missing data; only matching behavior changes.

### Step 4: Add The Postgres Partial Unique Index

Prompt:

```text
Add a Postgres-only migration or operator-run SQL script that enforces one active maintainer per normalized GitHub handle.

Do not rely on GORM tags alone. Use an explicit Postgres partial unique index.

Preferred SQL:

create unique index concurrently idx_maintainers_unique_normalized_github_active
on maintainers (lower(btrim(git_hub_account)))
where deleted_at is null
  and nullif(btrim(git_hub_account), '') is not null
  and lower(btrim(git_hub_account)) <> 'github_missing';

If the implementation adds a generated normalized_github column, add it through explicit SQL and expose it in model.Maintainer as read-only:

NormalizedGitHub string `gorm:"column:normalized_github;->"`

Before adding the index, add a preflight query that fails clearly if duplicates remain. The migration/script must not partially apply if the preflight fails.

Update local DB scripts if needed so restored local snapshots receive the same schema constraint.
```

Acceptance checks:

- The preflight query reports zero duplicate active normalized GitHub handles before index creation.
- The unique index exists in local Postgres after applying the script.
- Attempting to create a second active maintainer with the same normalized GitHub handle fails.
- Creating maintainers with blank/GITHUB_MISSING handles remains possible.
- Soft-deleting a duplicate permits a replacement active record if needed.

### Step 5: Split Emails And External Identities Into Child Tables

Prompt:

```text
Design and implement a first-pass identity model that separates a maintainer person from their email addresses and external identities.

Add Postgres-backed models and migrations for:
- maintainer_emails
- maintainer_identities

Suggested fields for maintainer_emails:
- id
- maintainer_id
- email
- normalized_email
- source, for example manual, github, lfx, cncf, fossa
- is_primary
- verified_at nullable
- first_observed_at
- last_observed_at
- created_at, updated_at, deleted_at

Suggested fields for maintainer_identities:
- id
- maintainer_id
- provider, for example github or lfx
- external_id nullable
- login_or_handle
- normalized_login_or_handle
- source
- first_observed_at
- last_observed_at
- created_at, updated_at, deleted_at

Uniqueness rules:
- active GitHub normalized login must be unique
- active LFX external ID/LFID must be unique when present
- email uniqueness should be conservative and source-aware; do not assume an email is globally permanent without review

Backfill from existing maintainer fields:
- maintainers.email -> maintainer_emails source=manual or legacy
- maintainers.github_email -> maintainer_emails source=github when usable
- maintainers.git_hub_account -> maintainer_identities provider=github
- maintainers.lfx_user_id -> maintainer_identities provider=lfx when usable
- maintainer_identity_observations -> child rows where useful

Keep existing columns during the first pass for compatibility with the current web UI and API. Add read/write helpers that maintain both old columns and new child tables until callers migrate.

Do not expose this broadly in the web UI in the first pass unless needed for validation.
```

Acceptance checks:

- Backfill is idempotent.
- Existing maintainer pages still render.
- Existing FOSSA and LFX flows still find the expected maintainer.
- The new child tables contain the expected identities for a sampled set of duplicate handles.
- Tests cover primary email selection and GitHub identity lookup.

## Cross-Cutting Requirements

- Treat production as Postgres-only.
- Keep data repair scripts explicit and operator-run.
- Add tests for identity matching before adding the database uniqueness constraint.
- Avoid unrelated UI changes.
- Keep audit logs useful: when a maintainer row is merged or an identity is linked, record what changed and why.
- Never use sentinel values as proof of identity.
