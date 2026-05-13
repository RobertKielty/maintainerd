# Dot-Project Adoption Plan

## Goal

Adopt CNCF `.project` repositories as the preferred source of project metadata in `maintainer-d`, starting with maintainer roster discovery and expanding to the broader metadata set defined by `cncf/automation`.

## Upstream Contract

As of 2026-05-06, the upstream CNCF documentation defines these `.project` files:

- `project.yaml` - required
- `MAINTAINERS.yaml` - recommended
- `SECURITY.md` - recommended
- `CONTRIBUTING.md` - recommended
- `GOVERNANCE.md` - recommended

The upstream README is inconsistent about `MAINTAINERS.yaml` vs `maintainers.yaml`, so `maintainer-d` should treat `MAINTAINERS.yaml` as canonical while accepting both names during migration.

`project.yaml.schema_version` is the authoritative upstream schema version and must be persisted with the imported metadata.

## Maintainer Roster Contract

For dot-project roll call, diffing, and PR generation, `maintainer-d` should treat the maintainer roster as:

- the `members` list from the team whose normalized `name` is exactly `project-maintainers`
- GitHub handles compared case-insensitively
- all other teams ignored for this purpose

If a `maintainers.yaml` file does not contain a `project-maintainers` team, that should be treated as a validation issue, persisted by the sync layer, and shown clearly in the UI.

## Working Principles

- Use a background sync job as the authoritative discovery mechanism.
- Avoid probing GitHub live during page render except as a temporary fallback if explicitly needed.
- Keep logical naming aligned with the new `.project` model even when underlying DB column names remain temporarily unchanged.
- Track importer version, last checked time, and file hashes so data can be reprocessed safely when either upstream files or local parsers change.

## Commit Plan

### Commit 1: Logical Rename and UI Framing

- Rename `DotProjectYamlRef` to `DotProjectMaintainerRef` in Go types, API payloads, and frontend types.
- Keep the underlying database column name as `dot_project_yaml_ref` for now via explicit GORM tags.
- Rename route labels:
  - `ROLL CALL` -> `LEGACY ROLL CALL`
  - `PROJECT RECORDS` -> `DOT-PROJECT ROLL CALL`
- Update create-form wording to refer to the dot-project maintainer file rather than a generic YAML field.

#### Implementation report

Step 1 is complete.

What changed:

- Renamed the logical maintainer ref field from `DotProjectYamlRef` to `DotProjectMaintainerRef` in the Go model, API payloads, and frontend types.
- Kept the underlying database column name as `dot_project_yaml_ref` so the logical rename did not require an immediate physical database rename.
- Renamed the project route labels in the web app:
  - `ROLL CALL` -> `LEGACY ROLL CALL`
  - `PROJECT RECORDS` -> `DOT-PROJECT ROLL CALL`
- Updated create/edit wording so the dot-project maintainer reference is described as a maintainer file rather than a generic YAML field.
- Updated the route-level BDD expectations to match the new navigation labels.

Operational details:

- This was intentionally a logical rename first, leaving the legacy column name in place so later commits could build on a stable schema.
- The visible effect of this step is limited to the renamed navigation labels and updated dot-project maintainer terminology.

Verification:

- `GOCACHE=/tmp/go-build go test ./...`
- `WEB_BDD_USE_MICROCKS=true BDD_FEATURE=../features/web/project_routes.feature make test-web`
- `WEB_BDD_USE_MICROCKS=true BDD_FEATURE=../features/web/maintainer_roll_call.feature make test-web`

### Commit 2: Add `.project` Ref Fields to the Project Model

Add first-class ref fields on `Project` for:

- `DotProjectRepoRef`
- `DotProjectProjectRef`
- `DotProjectMaintainerRef`
- `DotProjectSecurityRef`
- `DotProjectContributingRef`
- `DotProjectGovernanceRef`

Also add:

- `DotProjectSchemaVersion`
- `DotProjectMaintainerCount`
- `DotProjectLastSyncedAt`
- `DotProjectAdoptionStatus`

#### Implementation report

Step 2 is complete.

What changed:

- Added first-class dot-project fields to `model.Project`:
  - `DotProjectRepoRef`
  - `DotProjectProjectRef`
  - `DotProjectMaintainerRef`
  - `DotProjectSecurityRef`
  - `DotProjectContributingRef`
  - `DotProjectGovernanceRef`
  - `DotProjectSchemaVersion`
  - `DotProjectMaintainerCount`
  - `DotProjectLastSyncedAt`
  - `DotProjectAdoptionStatus`
- Exposed the new fields through the BFF project detail, project list, and search responses.
- Updated frontend project types to accept the expanded dot-project metadata surface.
- Cleaned the `ProjectRouteClient` prop usage so the updated frontend types still passed lint and typecheck cleanly.

Operational details:

- This step expanded `Project` so later discovery and sync commits could persist dot-project metadata directly on the main project row.
- The fields were added before any discovery logic so the web app could remain stable while the backend gained the storage shape needed for later steps.

Verification:

- `GOCACHE=/tmp/go-build go test ./...`
- `npm --prefix web run lint`
- `npm --prefix web run typecheck`
- `GOCACHE=/tmp/go-build BDD_FEATURE=../features/web/project_routes.feature make test-web`

### Commit 3: Add Dot-Project Sync State Storage

Create a sync/cache table keyed by project to hold:

- repo existence
- per-file existence flags
- canonical filename found for maintainers file
- schema version
- fetch hashes
- last checked time
- importer version
- sync and parse errors

#### Implementation report

Step 3 is complete.

What changed:

- Added `DotProjectSyncState` as a first-class model keyed by `ProjectID`.
- The sync-state table stores:
  - repo existence
  - per-file existence flags
  - the canonical maintainer filename found
  - default branch
  - schema version
  - importer version
  - per-file ETags
  - per-file body hashes
  - last checked time
  - sync error
  - parse error
- Added store methods to read and upsert dot-project sync-state rows.
- Registered the new table in migration, bootstrap, seed, and Go test schema setup paths.
- Added store tests covering nil lookup and upsert/reload behavior for dot-project sync state.

Operational details:

- This commit was storage-only. It introduced the dedicated sync/cache table used by later commits without adding new web behavior.
- The separation between `projects` summary fields and `dot_project_sync_states` was intentional so operational sync details could evolve independently from user-facing project metadata.

Verification:

- `GOCACHE=/tmp/go-build go test ./db ./cmd/web-bff ./cmd/fossa-poller ./cmd/migrate ./cmd/web-bff-seed`
- `GOCACHE=/tmp/go-build go test ./...`

### Commit 4: Build the Discovery and Parse Package

Add a reusable package that:

- resolves `.project` repos from `Project.GitHubOrg`
- checks the default branch
- discovers the supported files
- accepts both `MAINTAINERS.yaml` and `maintainers.yaml`
- parses `project.yaml`
- parses maintainers data and computes a deduplicated maintainer count
- records version-aware parse status

#### Implementation report

Step 4 is complete.

What changed:

- Added a reusable `dotproject` discovery package.
- The package:
  - resolves `.project` repos from `Project.GitHubOrg`
  - detects the default branch
  - discovers `project.yaml`, `MAINTAINERS.yaml`, `maintainers.yaml`, `SECURITY.md`, `CONTRIBUTING.md`, and `GOVERNANCE.md`
  - parses `project.yaml` for `schema_version`
  - parses maintainers data and computes a deduplicated maintainer count
  - records parse status as `missing`, `parsed`, `unsupported_schema`, `invalid_yaml`, or `invalid_shape`
- Added a `GitHubRepositoryClient` adapter over `go-github` so later sync jobs could call the package directly.
- Added focused discovery tests covering:
  - missing `.project` repos
  - lowercase `maintainers.yaml`
  - deduplicated maintainer counts
  - unsupported schema versions

Operational details:

- This step accepted both `MAINTAINERS.yaml` and `maintainers.yaml`, reflecting the current upstream inconsistency while keeping `MAINTAINERS.yaml` as the canonical preferred filename.
- The package was built to be reusable and testable in isolation before any persistence or UI wiring was added.
- This step did not add new visible web behavior.

Verification:

- `GOCACHE=/tmp/go-build go test ./dotproject`
- `GOCACHE=/tmp/go-build go test ./dotproject ./db ./cmd/web-bff`
- `GOCACHE=/tmp/go-build go test ./...`

### Commit 5: Backfill and Sync Job

- Add a job to iterate every project in the database.
- Persist discovered refs and sync state.
- Make the job idempotent and safe to rerun.

#### Implementation report

Step 5 is complete.

What changed:

- Added a reusable sync layer in `dotproject/sync.go` that:
  - iterates projects through a `Syncer`
  - calls the dot-project discovery package for each project
  - maps discovery results into both `projects` summary fields and `dot_project_sync_states`
  - classifies adoption status as `not_found`, `repo_only`, `partial`, `adopted`, or `error`
- Added a dedicated backfill command in `cmd/dot-project-sync/main.go` that:
  - opens the configured database
  - authenticates to GitHub using `GITHUB_API_TOKEN`
  - runs the sync across every project
  - prints a final status summary
- Added transactional persistence in the store layer:
  - `ListProjects`
  - `UpdateProjectDotProjectMetadata`
  - `PersistDotProjectSync`
  - the transactional path ensures project summary fields and sync-state rows are updated together
- Added store tests and sync tests to cover:
  - updating dot-project fields on `projects`
  - persisting sync-state rows
  - adopted and error outcomes
  - summary rollups across multiple projects
  - inferring `GitHubOrg` from `LegacyMaintainerRef` when older snapshots have an empty `git_hub_org`

Operational details:

- The sync job is idempotent in the intended sense: rerunning it recomputes discovery, refreshes the project summary fields, and upserts the matching `dot_project_sync_states` row.
- On transient GitHub errors it records sync failure state without wiping previously discovered project metadata.
- The sync path now falls back to deriving the GitHub org from `LegacyMaintainerRef` when `Project.GitHubOrg` is empty, which is important for older production snapshots.
- The legacy `dot_project_yaml_ref` database column is kept as `text` in the model so migrations do not conflict with the generated `search_tsv` column on refreshed production snapshots.
- The local refresh path now runs migrations automatically after restoring production data, so the local Podman database is ready for `dot-project-sync` immediately after refresh.

Verification:

- `GOCACHE=/tmp/go-build go test ./dotproject ./db ./cmd/dot-project-sync`
- `GOCACHE=/tmp/go-build go test ./...`

How to run locally against the refreshed Podman database:

```bash
go run ./cmd/dot-project-sync
```

The command expects:

- `GITHUB_API_TOKEN` to be set
- `MD_DB_DRIVER` and `MD_DB_DSN` to point at the target database when not using defaults

What to sanity check after it runs:

In the database:

```bash
./scripts/psql-local-maintainerd.sh "select count(*) from dot_project_sync_states;"
./scripts/psql-local-maintainerd.sh "select project_id, repo_exists, project_file_exists, maintainers_file_exists, schema_version, maintainers_filename, last_checked_at from dot_project_sync_states order by project_id limit 30;"
./scripts/psql-local-maintainerd.sh "select id, name, git_hub_org, dot_project_repo_ref, dot_project_project_ref, dot_project_yaml_ref, dot_project_schema_version, dot_project_maintainer_count, dot_project_adoption_status, dot_project_last_synced_at from projects where dot_project_last_synced_at is not null order by name limit 30;"
./scripts/psql-local-maintainerd.sh "select s.project_id, p.name, p.git_hub_org, s.sync_error from dot_project_sync_states s join projects p on p.id = s.project_id where s.sync_error is not null order by s.project_id limit 30;"
```

Useful spot checks:

- projects with `.project` repos should have `dot_project_repo_ref`
- projects with discovered `project.yaml` should have `dot_project_project_ref`
- projects with discovered maintainer files should have `dot_project_yaml_ref`
- `dot_project_sync_states.maintainers_filename` should show either `MAINTAINERS.yaml` or `maintainers.yaml`
- `dot_project_adoption_status` should be one of `not_found`, `repo_only`, `partial`, `adopted`, or `error`

In the web app:

- there is still no new visible UI for step 5 alone
- the app should look unchanged until step 6
- the main web sanity check is simply that existing project pages still load without regression

The next user-visible commit is step 6, where these persisted values start driving the navigation status and the legacy-page note.

### Commit 6: Surface Persisted Adoption Status in the UI

- Show persisted `.project` adoption state in project navigation and roll call views.
- Add the legacy-page note when a dot-project maintainer file exists.
- Add green tick and red X status indicators based on persisted discovery state.

#### Implementation report

Step 6 is complete.

What changed:

- Renamed the project route navigation labels to emphasize migration tracking:
  - `LEGACY ROLL CALL`
  - `DOT-PROJECT ROLL CALL`
- Surfaced persisted dot-project adoption state in the project route navigation.
- Added a legacy-page note when a persisted dot-project maintainer file exists.
- Added navigation status indicators:
  - green tick on `DOT-PROJECT ROLL CALL` when a dot-project repo is present
  - red X on `LEGACY ROLL CALL` when a dot-project repo is present
  - the inverse when the persisted adoption state is `not_found`
- Seeded route test data so the navigation status and migration note are visible in the default web BDD fixtures.
- Updated theme variables and component styles so the new UI callouts and status markers work in both light mode and dark mode.

Operational details:

- This step remained strictly persisted-data driven. The UI does not probe GitHub directly during page render.
- The dot-project route still remained a placeholder at the end of step 6, but it now surfaced the persisted adoption summary needed for the later full roll call page.

Verification:

- `npm --prefix web run lint`
- `npm --prefix web run typecheck`
- `GOCACHE=/tmp/go-build go test ./cmd/web-bff-seed`
- `WEB_BDD_USE_MICROCKS=true BDD_FEATURE=../features/web/project_routes.feature make test-web`

### Commit 7: Implement the Dot-Project Roll Call Page

Replace the current stub route with a page that shows:

- `.project` repo presence
- per-file presence
- schema version
- maintainer count
- last sync time
- links to discovered files

#### Implementation report

Step 7 is complete.

What changed:

- Replaced the dot-project placeholder content with a persisted dot-project roll call view.
- Extended the project detail API to include the persisted `DotProjectSyncState` summary for the selected project.
- Added a dot-project roll call section that shows:
  - persisted adoption status
  - last checked time
  - repo presence
  - schema version
  - maintainer count
  - maintainer filename
  - per-file presence for:
    - `.project` repo
    - `project.yaml`
    - `MAINTAINERS.yaml` / `maintainers.yaml`
    - `SECURITY.md`
    - `CONTRIBUTING.md`
    - `GOVERNANCE.md`
  - links to each discovered file when a persisted ref exists
  - recorded sync and parse errors when present
- Seeded a persisted sync-state row for `Project Atlas` so the dot-project route is testable end-to-end in the seeded web environment.
- Updated route-level BDD coverage to assert on the persisted roll call summary and the tracked file list.

Operational details:

- The page remains read-only and persisted-data driven. It renders the sync job output rather than probing GitHub live during page render.
- The route uses the project-level ref fields for links and the sync-state record for repo/file presence and error reporting.
- When no persisted sync state exists yet, the page explains that the dot-project background sync job must be run first.

Verification:

- `npm --prefix web run lint`
- `npm --prefix web run typecheck`
- `GOCACHE=/tmp/go-build go test ./cmd/web-bff ./cmd/web-bff-seed`
- `WEB_BDD_USE_MICROCKS=true BDD_FEATURE=../features/web/project_routes.feature make test-web`

What to sanity check locally:

- Run `go run ./cmd/dot-project-sync` against the refreshed local Podman database.
- Open `/projects/<id>/dot-project` for a project with persisted dot-project state.
- Confirm the page shows:
  - adoption status
  - last checked time
  - schema version
  - maintainer count
  - the tracked file table with `FOUND` / `MISSING` badges
  - links for any discovered dot-project files
- Open a project without a dot-project repo and confirm the route shows the persisted `not_found` state cleanly rather than the old placeholder.

### Deferred Work: `project.yaml` Metadata Import

The original next step was to import selected `project.yaml` metadata such as:

- `slug`
- `description`
- `website`
- `repositories`
- `cncf_slack_channel`
- `mailing_lists`
- selected governance and security references

This work is now deferred until after the dot-project maintainer-file UX and PR workflow are in place.

### Reporting and Search

Add search/reporting for:

- projects with `.project` repos
- projects with `project.yaml`
- projects with a maintainer roster
- projects missing recommended files
- schema version distribution
- maintainer coverage comparisons between legacy and dot-project sources

#### Partial implementation report

Initial reporting work is complete.

What changed:

- Added dot-project repo filtering to the main project list.
- The main list now supports:
  - `All`
  - `adopted!`
  - `legacy file`
- Wired the filter through the recent-projects API so it remains server-side and works cleanly with existing pagination.
- Corrected the main list `.project repo` column so it links to the `.project` repo itself rather than the maintainer file.
- Updated the mobile project card view to show the `.project` repo link consistently.
- Added focused BDD coverage for the dot-project repo filter states.

Operational details:

- This is the first slice of the broader reporting/search work planned for commit 9.
- The current filter is based on persisted `.project` repo presence using the project-level dot-project ref fields.
- The user-facing labels intentionally match migration framing:
  - `adopted!` for projects with a `.project` repo
  - `legacy file` for projects without a `.project` repo

Verification:

- `GOCACHE=/tmp/go-build go test ./cmd/web-bff`
- `npm --prefix web run lint`
- `npm --prefix web run typecheck`
- `WEB_BDD_USE_MICROCKS=true BDD_FEATURE=../features/web/project_list_dot_project_filter.feature make test-web`

## Phase 2 Plan

### Commit A: Deploy Hourly Dot-Project Sync CronJob

- Deploy `cmd/dot-project-sync` as a Kubernetes `CronJob`.
- Run once per hour.
- Only scan active projects.
- Exclude the `maintainer-d` project from scanning.
- Record an audit-log summary for each run.
- Include GitHub API failures, including rate-limit failures, in the summary entry.

Operational goals:

- Keep the background sync authoritative for adoption detection.
- Make failures visible to staff without requiring direct pod log access.

#### Implementation report

Commit A is complete.

What changed:

- Added a dedicated `maintainerd-dot-project-sync` container target to the main `Dockerfile`.
- Added Makefile targets for:
  - building the dot-project sync image
  - pushing the dot-project sync image
  - applying the dot-project sync CronJob
  - triggering a manual dot-project sync job
  - setting the deployed CronJob image
- Added a dedicated Kubernetes manifest:
  - `deploy/manifests/dot-project-sync-cronjob.yaml`
- Configured the CronJob to run hourly.
- Updated the syncer so it only scans active projects.
- Explicitly excluded the `maintainer-d` project from sync scans.
- Extended sync summaries so they now track:
  - loaded projects
  - scanned projects
  - skipped projects
  - archived skips
  - explicit exclusions
  - GitHub API error counts
  - rate-limit error counts
  - summarized run errors
- Added a run-level audit-log entry with action `DOT_PROJECT_SYNC_RUN`.
- Included GitHub/rate-limit failures in the audit metadata summary.
- Prefixed each sync error in the audit metadata with the project name so the log can be traced back to the failing project directly.
- Added post-sync Postgres size and coverage metrics to the same audit metadata summary:
  - total database size
  - `dot_project_sync_states` total relation size
  - cached maintainer-body count
  - cached maintainer-body bytes, average bytes, and max bytes
  - sync-state coverage counts for total rows, repos found, and cached bodies

Operational details:

- The older generic `maintainer-sync` CronJob and image remain unchanged.
- The dot-project sync deployment is intentionally separate so it can evolve independently from the Kubernetes resource sync job.
- The run summary is now visible through the existing audit log view instead of only through pod logs.
- The post-sync DB size metrics are collected directly from the production Postgres database used by maintainer-d.

Verification:

- `GOCACHE=/tmp/go-build go test ./dotproject ./cmd/dot-project-sync`
- `GOCACHE=/tmp/go-build go test ./...`

### Commit B: Cache Raw Dot-Project File Bodies

- Persist the raw cached body for the discovered `maintainers.yaml` file in maintainer-d.
- Accept any filename casing variant, but keep the file plural: `maintainers.yaml`.
- Prefer cached file bodies in the web UI instead of fetching GitHub live on page load.
- Expose the cached maintainer-file body and related metadata through the project detail API.

#### Implementation report

Commit B is complete.

What changed:

- Extended `dot_project_sync_states` to persist the raw cached maintainer-file body alongside the existing maintainer-file hash and ETag.
- Updated dot-project discovery to accept any root-level filename casing variant of `maintainers.yaml` while preserving the actual filename found.
- Persisted the cached maintainer-file body during `dot-project-sync`.
- Exposed the cached maintainer-file payload through the project detail API, including:
  - filename
  - ETag
  - body hash
  - cached body
  - last checked time
- Updated the dot-project roll call UI to render the cached maintainer file from persisted state instead of relying on a live GitHub fetch.
- Seeded cached maintainer-file body data for the web test fixture and added focused tests for:
  - mixed-case maintainer filename discovery
  - persisted maintainer-file body storage
  - project detail API exposure of cached maintainer-file metadata

Operational details:

- The new cache is persisted only for the dot-project maintainer file, not for the other recommended `.project` files.
- The sync path remains authoritative: the web UI renders the cached maintainer body captured by the background sync job.
- Filename matching is case-insensitive for discovery, but the implementation still treats the plural filename `maintainers.yaml` as the supported contract.

Verification:

- `GOCACHE=/tmp/go-build go test ./dotproject ./db ./cmd/web-bff ./cmd/dot-project-sync`
- `npm --prefix web run lint`
- `npm --prefix web run typecheck`

Operational goals:

- Make the UI deterministic and resilient to GitHub availability.
- Provide a stable source of truth for roll-call diffing and PR preview generation.

### Commit C: Render a Formatted Dot-Project Maintainer File

- Render the cached `maintainers.yaml` body in the dot-project route.
- Use a structured, AST-backed YAML renderer rather than a generic syntax highlighter.
- Colorize the YAML while preserving existing whitespace and comments in the rendered view.
- Keep the first version read-only.

Operational goals:

- Establish a strong file-viewing experience before adding inline diff and PR behaviors.
- Preserve enough source fidelity that later edit/PR generation can patch the original file cleanly.

#### Implementation report

Commit C is complete.

What changed:

- Added a dedicated formatted maintainer-file viewer for the dot-project route.
- Parses the cached YAML with an AST-backed renderer path before deriving team and member structure.
- Shows a read-only structured team/member summary above the source view.
- Renders the cached source text with YAML-aware coloring while preserving whitespace and comments.
- Stopped trimming the cached maintainer-file body before rendering, so source fidelity is retained for later patch/diff work.
- Added route-level BDD coverage for the formatted cached maintainer-file view.

Operational details:

- The view remains read-only; maintainer-d matching, inline validation, diffing, and PR behavior are left for later commits.
- The source of truth remains the cached body populated by `dot-project-sync`, not a live GitHub fetch during page render.

Verification:

- `npm --prefix web run lint`
- `npm --prefix web run typecheck`
- `GOCACHE=/tmp/go-build go test ./cmd/web-bff-seed`

### Commit D: Add Inline Maintainer-D Awareness to the Rendered YAML

- Match GitHub handles case-insensitively against active maintainer records in maintainer-d.
- Make matched GitHub handles link to the maintainer route in maintainer-d.
- Underline unmatched GitHub handles with a red wavy line.
- Clicking an unmatched handle should open the existing `Add Maintainer to CNCF INTERNAL DB` flow, prefilled with the GitHub handle when possible.
- Surface the absence of a `project-maintainers` team as a validation issue in the dot-project UI.

Operational goals:

- Move missing-maintainer feedback into the rendered file itself.
- Keep the interaction clearly focused on the standardized `project-maintainers` team.

#### Implementation report

Commit D is complete.

What changed:

- Added maintainer-d awareness to the formatted dot-project maintainer-file viewer.
- Matched `project-maintainers` members case-insensitively against active project maintainer records.
- Rendered matched handles as links to their maintainer routes.
- Rendered unmatched `project-maintainers` handles with a red wavy underline.
- Wired unmatched handle clicks to the existing `Add Maintainer to CNCF INTERNAL DB` modal, prefilled with the GitHub handle and source line.
- Added a validation message when the cached file does not contain a `project-maintainers` team.
- Extended the web BDD fixture with an unmapped dot-project handle and covered the matched-link and add-modal paths.

Operational details:

- The inline awareness is intentionally limited to the standardized `project-maintainers` team.
- The file view remains read-only; DB-vs-file diffing and PR generation are still deferred to later commits.

Verification:

- `npm --prefix web run lint`
- `npm --prefix web run typecheck`
- `GOCACHE=/tmp/go-build go test ./cmd/web-bff-seed`
- `WEB_BDD_USE_MICROCKS=true BDD_FEATURE=../features/web/project_routes.feature make test-web`

### Commit E: Add DB-vs-File Diff Summary and PR Preview

- Compare active project maintainers in maintainer-d against the members of the `project-maintainers` team only.
- Show active maintainers that exist in maintainer-d but are missing from `maintainers.yaml`.
- Add a `Create Pull Request` action that patches the rendered file in the UI preview with the missing GitHub handles.
- Render a suggested updated `maintainers.yaml` preview that respects the existing comments, formatting, and ordering as much as possible.

Operational goals:

- Replicate the existing roll-call diff behavior for dot-project, but in a better UI.
- Keep the preview faithful to the original file so maintainers can trust the generated patch.

### Commit F: Submit Pull Request Workflow

- Add a `Submit Pull Request` action after the preview patch has been generated.
- Create a branch and PR against the project’s `.project` repository using maintainer-d’s service credentials.
- Record the PR creation in the audit log.
- Show PR result details back in the UI.

Operational goals:

- Turn missing-maintainer detection into an actionable remediation workflow.
- Keep the write path bot-driven first rather than depending on each end user’s GitHub OAuth identity.

### Commit G: Return to Deferred `project.yaml` Metadata Import

- Resume the selected `project.yaml` import work after the maintainer-file UX and PR workflow are in place.

### Commit H: Finish the Remaining Reporting and Search Work

- add reporting for projects with `project.yaml`
- add reporting for projects with a maintainer roster
- add reporting for projects missing recommended files
- add schema version distribution reporting
- add legacy-vs-dot-project maintainer coverage comparisons

## Schema Tracking

Track three distinct versions:

1. Upstream schema version from `project.yaml.schema_version`
2. Observed file layout, especially the canonical vs tolerated maintainer filename
3. Local importer version used by `maintainer-d`

Persist per-file hashes so reprocessing can be triggered when either source data changes or importer logic changes.
