# Auto-Add Maintainer Safeguard

## Goal

Add a guarded path for `cmd/dot-project-sync` to automatically add missing maintainers discovered from `.project/maintainers.yaml` into the maintainer-d database.

The long-term source of truth is the `project-maintainers` team in each project's `.project/maintainers.yaml` file. During the current rollout, that source is still being generated and refined, so automatic addition must require a second gate: the GitHub account must also be present in `cncf/foundation` `project-maintainers.csv`.

## Current Context

`dot-project-sync` already:

- discovers `.project` repositories for known projects
- caches each project's `maintainers.yaml` body in `dot_project_sync_states`
- parses GitHub handles from `maintainers[].teams[]` entries named `project-maintainers`
- enriches discovered handles through required LFX identity observation writes

`db.SQLStore.UpsertMaintainer` already supports creating or linking a maintainer to a project by GitHub handle/email/company, but `dot-project-sync` does not currently use it to add newly discovered handles.

## Proposed Gates

Treat a GitHub account as eligible for automatic maintainer addition only when both gates pass:

1. **Dot-project gate**
   - The GitHub account appears in the `members` list of a team named `project-maintainers`.
   - The team must be found under the project's `.project/maintainers.yaml` `maintainers.teams` hierarchy.
   - Existing parser behavior lowercases handles and strips an optional leading `@`.

2. **Foundation CSV gate**
   - The same GitHub account appears under the same project in `https://github.com/cncf/foundation/blob/main/project-maintainers.csv`.
   - The job should fetch/cache the CSV once per run.
   - Matching should be case-insensitive and trim whitespace.
   - The CSV should be parsed with Go's `encoding/csv`, not ad hoc string splitting.
   - The CSV's `Project` value does not repeat on every row; parsing must carry the most recent non-empty project value forward until a new project value appears.
   - The code that caches the content of the Foundation CSV file should store the line number of the row for each GitHub account found there so that it can be used in the audit log entry that refers to the this file as the evidence that corroborates CNCF-registered-maintainership

The feature should be controlled by a command-line flag:

```text
--check-foundation-csv
```

Default: `true`.

When `--check-foundation-csv=true`, missing handles from `.project/maintainers.yaml` must not be added unless they are present in the foundation CSV.

When `--check-foundation-csv=false`, the job may use only gate A. This should be treated as an explicit operator override and reflected in audit metadata.

Automatic writes should be controlled by a second command-line flag:

```text
--auto-add-maintainers
```

Default: `false`.

When `--auto-add-maintainers=false`, the job should report and audit what would have been created or linked, but must not write to `maintainers` or `maintainer_projects`.

When `--auto-add-maintainers=true`, the job may create/link maintainers that pass the active gates.

## Foundation CSV Cache

Add a small foundation CSV loader for `dot-project-sync`:

- default repository source: `cncf/foundation` `main:project-maintainers.csv`
- configurable GitHub source for tests/local operation:
  - `--foundation-csv-owner`, default `cncf`
  - `--foundation-csv-repo`, default `foundation`
  - `--foundation-csv-ref`, default `main`
  - `--foundation-csv-path`, default `project-maintainers.csv`
- request timeout should use the job context
- fetch once per process and share the parsed index across all projects
- fail closed when the CSV cannot be loaded or parsed and `--check-foundation-csv=true`
- use the GitHub API with the job's existing `GITHUB_API_TOKEN` to retrieve the CSV content and source commit/blob metadata once per run

Suggested parsed record shape:

```go
type FoundationMaintainerRecord struct {
    Project     string
    Name        string
    Company     string
    GitHub      string
    LineNumber  string
    CommitSHA   string
}
```

The in-memory index must support lookup by normalized project plus normalized GitHub account. A global GitHub-only index is not sufficient because a person may be a registered maintainer on one CNCF project but not another.

Project name matching first-pass rule: require an exact project-name match after trimming whitespace. Do not use global handle membership, fuzzy matching, or aliases in the first pass. If maintainer-d project names and foundation CSV project names differ, skip the automatic add/link and report the mismatch through summary metrics and audit metadata.

The CSV source should be resolved to and recorded with the exact `cncf/foundation` commit SHA used for the run. Observations and audit metadata should include the raw CSV URL and commit SHA so decisions are reproducible.

## Identity Observation

Membership of `foundation/project-maintainers.csv` should be recorded as `MaintainerIdentityObservation`.

Proposed values:

- `source`: `foundation-csv`
- `source_ref`: `github:<normalized-handle>` or `foundation-csv:<normalized-handle>`
- `source_user_id`: blank unless we decide the CSV row has a stable identifier, which it currently does not appear to have
- `name`: value from `Maintainer Name`
- `git_hub_user`: normalized GitHub account
- `company_name`: value from `Company`
- `match_status`: `matched` when present in the CSV, `unmatched` when checked and absent
- `match_reason`: concise gate result, for example `present in cncf/foundation project-maintainers.csv`
- `confidence`: `strong`
- `raw_payload`: JSON object with the CSV row, URL, and exact foundation commit SHA
- `observed_at`: sync time
- `maintainer_id`: set when the handle maps to an existing or newly inserted maintainer
- `project_id`: set for the project whose dot-project maintainer file triggered the check

Persist `unmatched` observations for handles found in `.project/maintainers.yaml` but absent from the foundation CSV for that project. These observations must not result in writes to `maintainers` or `maintainer_projects`.

Dry-run mode (`--auto-add-maintainers=false`) should still write `MaintainerIdentityObservation` rows and an aggregate audit event. It must not write maintainer rows or maintainer-project relationships.

## Auto-Add Behavior

For each project with a parsed `project-maintainers` team:

1. Normalize and de-duplicate handles from `.project/maintainers.yaml`.
2. Resolve existing maintainers by case-insensitive GitHub handle.
3. For handles already linked to the project, do nothing except optionally record/update the foundation CSV observation.
4. For handles that map to an existing maintainer but are not linked to the project:
   - only link the maintainer to the project when the foundation CSV gate passes
   - when `--auto-add-maintainers=false`, report that the maintainer would have been linked but do not write the relationship
   - log an audit event for the new project membership
5. For handles that do not map to an existing maintainer:
   - only create the maintainer when both gates pass
   - populate fields from foundation CSV when available:
     - `Name` from `Maintainer Name`
     - `GitHubAccount` from `Github Name`
     - `Company` from `Company`
     - `Email` from LFX when available, otherwise `EMAIL_MISSING`
     - `GitHubEmail` from LFX when available and appropriate, otherwise `GITHUB_EMAIL_MISSING`
     - `LFXUserID` from LFX when an exact or strong match is found
     - `MaintainerStatus` = `Active`
   - after both gates pass, search LFX for the maintainer email before creating the row
   - if LFX does not return an exact or strong match, does not return an email, or cannot find the maintainer, create with `EMAIL_MISSING`
   - when `--auto-add-maintainers=false`, report that the maintainer would have been created but do not write the row or project relationship
   - associate the maintainer to the project
   - log an audit event for the automatic creation and project association

GitHub handle comparisons must be case-insensitive. For display, preserve the best mixed-case value available, preferably from the foundation CSV or, if that is blank, from `.project/maintainers.yaml`. Database joins and all GitOps-source comparisons should use normalized case-insensitive keys.

LFX lookup is required for the job. A maintainer that has not created or connected an LFX profile can still be reported as unmatched, but an LFX Platform access failure, such as an expired short-lived token, should stop the job and instruct the operator to refresh the token at `https://app.lfx.dev/settings`. Add an LFX user ID field to the canonical `Maintainer` record so compliance reports can show who has an LFX profile and who does not:

```go
LFXUserID string `gorm:"size:128;index"`
```

Use LFX email only for `exact` or `strong` LFX matches:

- `exact`: LFX user search resolves to a single user and `GetUserIdentities` confirms a linked GitHub identity whose source is `github` and whose username matches the maintainer GitHub handle case-insensitively.
- `strong`: LFX user search resolves to a single user through a strong identifier, but without verified GitHub identity confirmation. Examples: the searched email equals the returned user email case-insensitively, or GitHub-handle search returns exactly one user but the identity endpoint does not confirm a GitHub username.

Do not use `weak`, `ambiguous`, `unmatched`, or `error` LFX results for automatic email population.

## Audit Logging

Each automatic addition of a new maintainer should write an audit event. The event should make clear that the dot-project sync job manually launced by a human staff user, performed the change.

The UX here is very important, when an user looks at an audit event it is important that they can click on the referenced resources where possible, GitHub handles should link to GitHub profiles, references to maintainers.yaml file should link to the project's maitnainers.yaml, a maintainer-d profile should also be referenceable via a browser click. We cannot reference an LFX Profile at this time, so for that just note the LFX ID.

Audit fields:

- `Actor`: `dot-project-sync started by {END-USER}` replacing {END-USER} with the name of the signed in maintainer-d end user.
- `Action`: `ADD_DOT_PROJECT_MAINTAINER`
- metadata:
  - project name
  - GitHub handle
  - whether the maintainer row was created or only linked
  - dry-run vs write mode
  - foundation CSV gate enabled/disabled
  - foundation CSV URL
  - foundation CSV commit SHA
  - foundation CSV project/name/company values
  - LFX lookup result for email enrichment when applicable
  - dot-project maintainer file URL/blob ref
  - source: `.project/maintainers.yaml project-maintainers`

When we automatically add a new maintainer to the maintainer-db using this process the following audit log needs to be recorded.

In the following log statement, text within [] is English text where the | symbol indicates optionality, infer the choice of options from this specification. Text withing {} indicates a named resource should be presented as a HTML href clickable by the end-user of the maintainer-d web application

The {GitHub Handle} [was|would have been] added to maintainer-d as a [new maintainer|existing maintainer linked] found in {project-org}/.project/{maintainers.yaml}/{project-maintainers} source reference
corroborated-by {link to line that has maintainers github handle in the foundation csv}. Email address was [already known to maintainer-d|was found in their LFX Profile ID {LFX-Profile-ID}] [New|Updated] {ref to maintainer's profile in maintainer-d}

## Summary Metrics

Extend `dotproject.SyncSummary` and the final log line with counts such as:

- auto-add candidates checked
- auto-add dry-run candidates checked
- auto-add created new maintainers in db
- auto-add existing maintainers in db linked to new project
- auto-add would-create maintainers
- auto-add would-link existing maintainers
- auto-add skipped because foundation CSV missing
- auto-add skipped because CSV load/parse failed
- auto-add skipped because project CSV row missing
- auto-add LFX email lookups attempted/matched/unmatched/errored
- auto-add audit failures

These counts should also be included in the sync job's final aggregate audit event.

In dry-run mode, the aggregate audit event metadata should include the full would-create and would-link handle lists, grouped with the maintainer-d project name. If this becomes too large in practice, we can add truncation later with explicit counts, but the first pass should favor audit usefulness.

## Safety Defaults

- Foundation CSV gate defaults to enabled.
- Auto-add writes default to disabled.
- If the CSV cannot be loaded and the gate is enabled, do not create/link maintainers.
- If the project name does not exactly match a carried-forward foundation CSV project name after trimming, do not create/link maintainers.
- Auto-add should be idempotent.
- GitHub handle matching should be case-insensitive.
- Preserve mixed-case GitHub handles for display while matching through normalized keys.
- LFX profile lookup misses are non-blocking, but LFX Platform access failures block the sync. The job requires a valid `LFX_AUTH_TOKEN`; if the token is missing or the LFX API rejects a lookup, the job should stop and instruct the operator to refresh the token.
- Use LFX email and `LFXUserID` only from exact or strong LFX matches.
- Avoid adding placeholder handles such as `github-handle`.
- Do not overwrite richer existing maintainer fields with blank CSV values.
- Do not add inactive archived projects unless the project already passes existing `dot-project-sync` project filters.

## Implementation Sketch

1. Add CLI flag parsing to `cmd/dot-project-sync`.
   - Keep env vars currently used for LFX intact.
   - Add `--check-foundation-csv`, default `true`.
   - Add `--auto-add-maintainers`, default `false`.
   - Add configurable foundation CSV source flags:
     - `--foundation-csv-owner`
     - `--foundation-csv-repo`
     - `--foundation-csv-ref`
     - `--foundation-csv-path`
2. Add a foundation CSV fetcher/parser package or small internal dotproject helper.
   - Carry the last non-empty `Project` cell forward while parsing rows.
   - Build a project+GitHub index.
   - Use the GitHub API and existing `GITHUB_API_TOKEN` to fetch the file once per run.
   - Resolve and expose the exact source commit SHA.
3. Extend `dotproject.Syncer` with an optional auto-add component.
4. Extend the store interface with focused methods if needed:
   - lookup maintainer by GitHub handle
   - create/link maintainer with project in one transaction
   - write per-maintainer audit event
   - write foundation CSV identity observation
5. Add unit tests for:
   - CSV parser
   - carried-forward CSV project names
   - project-specific CSV membership checks
   - case-insensitive handle matching
   - missing CSV entry blocks creation
   - disabled CSV gate allows creation only when auto-add is enabled
   - dry-run reports would-create/would-link without writes
   - dry-run writes observations and aggregate audit metadata
   - existing maintainer is linked but not duplicated
   - LFX email lookup fallback to `EMAIL_MISSING`
   - exact and strong LFX matches can populate email and `LFXUserID`
   - weak, ambiguous, unmatched, and error LFX results do not populate email
   - audit metadata includes automatic dot-project source

## Clarifying Questions

None currently blocking implementation.
