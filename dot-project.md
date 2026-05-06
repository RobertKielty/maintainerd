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

### Commit 4: Build the Discovery and Parse Package

Add a reusable package that:

- resolves `.project` repos from `Project.GitHubOrg`
- checks the default branch
- discovers the supported files
- accepts both `MAINTAINERS.yaml` and `maintainers.yaml`
- parses `project.yaml`
- parses maintainers data and computes a deduplicated maintainer count
- records version-aware parse status

### Commit 5: Backfill and Sync Job

- Add a job to iterate every project in the database.
- Persist discovered refs and sync state.
- Make the job idempotent and safe to rerun.

### Commit 6: Surface Persisted Adoption Status in the UI

- Show persisted `.project` adoption state in project navigation and roll call views.
- Add the legacy-page note when a dot-project maintainer file exists.
- Add green tick and red X status indicators based on persisted discovery state.

### Commit 7: Implement the Dot-Project Roll Call Page

Replace the current stub route with a page that shows:

- `.project` repo presence
- per-file presence
- schema version
- maintainer count
- last sync time
- links to discovered files

### Commit 8: Import Selected `project.yaml` Metadata

Start with high-value fields:

- `slug`
- `description`
- `website`
- `repositories`
- `cncf_slack_channel`
- `mailing_lists`
- selected governance and security references

### Commit 9: Reporting and Search

Add search/reporting for:

- projects with `.project` repos
- projects with `project.yaml`
- projects with a maintainer roster
- projects missing recommended files
- schema version distribution
- maintainer coverage comparisons between legacy and dot-project sources

## Schema Tracking

Track three distinct versions:

1. Upstream schema version from `project.yaml.schema_version`
2. Observed file layout, especially the canonical vs tolerated maintainer filename
3. Local importer version used by `maintainer-d`

Persist per-file hashes so reprocessing can be triggered when either source data changes or importer logic changes.
