# Spec: Per-Project Maintainer Status

Status: implementation in progress on this branch/PR.
Branch: `fix/per-project-maintainer-status` (created from `main`).

## Problem

`Maintainer.MaintainerStatus` (`model/main.go:94`) is a single column on the `Maintainer`
row. A maintainer can belong to many `Project`s via the `maintainer_projects` many2many join
table, but that join table (`MaintainerProject`, `model/main.go:188-194`) carries no status —
only `MaintainerID`, `ProjectID`, `JoinedAt`.

Consequence: setting a maintainer's status on one project overwrites their status
everywhere. Confirmed reproduction: a maintainer active on Project B is set to `Emeritus` on
Project A (e.g. via the FOSSA setup flow in the web UI) → their global `MaintainerStatus`
row flips to `Emeritus` → every other project reading that field (FOSSA invites, service
invitations, LFX enrichment, roster generation, search) now treats them as non-active on
Project B too, even though nothing about their standing on Project B changed.

The UI itself invites this mistake: `ProjectRouteClient.tsx:574` calls
`POST /api/maintainers/status` (a bulk, maintainer-scoped update) from *inside a project's own
page* — a staff member reasonably reads that action as "set this person's status on this
project," not "set their status everywhere."

## Current model

```go
type Maintainer struct {
    gorm.Model
    ...
    MaintainerStatus MaintainerStatus `gorm:"type:text"`
    Projects         []Project `gorm:"many2many:maintainer_projects;..."`
}

type MaintainerProject struct {
    MaintainerID uint      `gorm:"primaryKey;index"`
    ProjectID    uint      `gorm:"primaryKey;index"`
    JoinedAt     time.Time `gorm:"autoCreateTime"`
    Maintainer   Maintainer `gorm:"foreignKey:MaintainerID;constraint:OnDelete:CASCADE"`
    Project      Project    `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
}
```

## Proposed model

Move status onto the join row, since status is a property of the maintainer-project
relationship, not of the maintainer alone:

```go
type MaintainerProject struct {
    MaintainerID uint             `gorm:"primaryKey;index"`
    ProjectID    uint             `gorm:"primaryKey;index"`
    Status       MaintainerStatus `gorm:"type:text;default:Active"`
    JoinedAt     time.Time        `gorm:"autoCreateTime"`
    Maintainer   Maintainer `gorm:"foreignKey:MaintainerID;constraint:OnDelete:CASCADE"`
    Project      Project    `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
}
```

`Maintainer.MaintainerStatus` is retired as a source of truth. Two options for what (if
anything) replaces it as a maintainer-level summary value — see "Open questions."

## Migration

1. **Additive schema change first.** `AutoMigrate` adds `maintainer_projects.status`
   (default `Active`) without touching `maintainers.maintainer_status` yet. No downtime, no
   backfill risk in this step.
2. **Backfill.** One-time migration: for every `MaintainerProject` row, set
   `status = maintainers.maintainer_status` from the currently-linked maintainer. This
   preserves today's (already-wrong) values as the starting point rather than resetting
   everyone to Active — the fix is forward-looking, not a retroactive data correction. Any
   cross-project cases already miscategorized by the bug (e.g. someone wrongly marked
   Archived globally because of an unrelated project) stay wrong until someone reviews them
   per-project; this migration doesn't have enough information to know which project's value
   was "correct."
3. **Dual-read / cutover.** Since this is a small, internally-used system (not a public API
   with external consumers to keep in lockstep), a phased dual-write period is unnecessary
   complexity. Land the store/API/call-site changes (below) in the same PR as steps 1-2, cut
   over atomically, and keep `maintainers.maintainer_status` column in place but unused for
   one release as a safety net before dropping it in a follow-up migration.
4. **Drop column.** Separate, later migration once the cutover has been running cleanly:
   remove `maintainers.maintainer_status` and the `MaintainerStatus` field from `Maintainer`.

## Store layer changes (`db/store.go`, `db/store_impl.go`)

| Current | Change |
|---|---|
| `UpdateMaintainerStatus(maintainerID uint, status)` | Replace with `UpdateMaintainerProjectStatus(maintainerID, projectID uint, status)` — updates the one join row. |
| `UpdateMaintainersStatus(ids []uint, status)` | Replace with `UpdateMaintainersProjectStatus(maintainerIDs []uint, projectID uint, status)` — bulk update scoped to one project, matching how `ProjectRouteClient.tsx` actually calls it today (from within a project page). |
| `UpdateMaintainerDetails(maintainerID, ..., status, companyID)` | Drop the `status` parameter — this handler edits maintainer-level fields (name/email/github/company), which are genuinely global; status no longer belongs here. |
| `GetMaintainersByProject(projectID)` | Preload/join `MaintainerProject.Status` so callers can filter/display per-project status without a second query. |
| `GetMaintainerMapByEmail()` (used by `cmd/sync`) | See CRD section below — this method assumes one status per maintainer, which no longer holds. |
| New: `GetMaintainerProjectStatus(maintainerID, projectID) (MaintainerStatus, error)` | For call sites that need a single project's status for one maintainer (e.g. per-project FOSSA invite gating). |

## Call sites requiring updates

Grouped by what they need to become project-scoped:

**Status mutation**
- `cmd/web-bff/main.go:3641` `handleMaintainerStatusUpdate` (route `POST /api/maintainers/status`, `main.go:315`) — must take a `projectID` (from request body or URL) and call the new project-scoped store method.
- `cmd/web-bff/main.go:2370-2382` PATCH `/api/maintainers/{id}` handler — remove status from this handler entirely (see table above); status edits move to the project-scoped endpoint.
- `cmd/sanitize/main.go:110-126` — currently reads project `p`'s maintainer roster and calls the global `UpdateMaintainerStatus` per maintainer. This is the actual cascade mechanism the user hit. Fix: call `UpdateMaintainerProjectStatus(m.ID, p.ID, ...)` — status changes stay scoped to the project whose roster was just parsed.

**Status filtering ("is this maintainer active *on this project*")**
- `cmd/web-bff/main.go:1750` `activeProjectMaintainerGitHubHandles` — already takes a `[]model.Maintainer` in a project context; needs the project ID threaded through to check per-project status instead of `maintainer.MaintainerStatus`.
- `cmd/web-bff/main.go:5753` `handleFossaInvite` — the exact path the user was in when they discovered this. Must filter on that project's `MaintainerProject.Status`, not the maintainer's global field.
- `cmd/web-bff/main.go:5803` `handleFossaTeamSync` — same fix.
- `cmd/web-bff/main.go:6030` `handleServiceInvitation` — same fix.
- `cmd/web-bff/main.go:6410`, `:6455` `handleSearchMaintainers` — search is cross-project by nature; needs a decision on what "status" means in an unscoped search result (see open questions).
- `lfx/enricher.go:147-149` — filters candidates by `maintainer.MaintainerStatus != Active` when `EnrichAll` is set; this is a global enrichment sweep, not project-scoped, so also needs the open-question decision below.

**Initialization (no per-project ambiguity — these create a maintainer with no projects yet, or link one project at creation time)**
- `db/store_impl.go:254` — sets `MaintainerStatus: model.ActiveMaintainer` on brand-new `Maintainer` creation. Becomes: create the `MaintainerProject` link (if a project is known at creation time) with `Status: Active`; drop the field from the `Maintainer{}` literal.
- `db/bootstrap.go:249`, `cmd/web-bff-seed/main.go` (8 sites) — same pattern; update to set status on the join row created alongside each maintainer/project link.

## Kubernetes CRD sync (`cmd/sync/main.go`, `apis/maintainers/v1alpha1/types.go`)

This is the least straightforward part. `apis.Maintainer` is a **per-maintainer** CRD object
(one per email, `cmd/sync/main.go:206` keyed by `store.GetMaintainerMapByEmail()`), with a
single `Status MaintainerLifecycle` field (`types.go:57`) — the same global assumption,
reproduced in the CRD schema. Making status per-project in Postgres doesn't fix this layer by
itself. Options, not yet decided:

1. Add a `Status` field per-entry to whatever CRD represents project-maintainer membership
   (needs confirming whether one exists today — `cmd/sync` wasn't in the grounding pass for
   this spec) and drop `Status` from `apis.Maintainer`.
2. Keep `apis.Maintainer.Status` but redefine it as a derived summary (e.g. "Active if active
   on any project") — cheaper, but reintroduces exactly the kind of misleading global signal
   this fix is meant to remove, just downstream.
3. Leave `apis.Maintainer.Status` deprecated/unused until CRD consumers are surveyed.

Recommend deferring this to a follow-up PR after the Postgres-side fix lands, once it's clear
who/what actually reads `apis.Maintainer.Status` today (out of scope for this repo's own
grounding — the CRDs are consumed by `apis/`/`config/` tooling this repo doesn't fully own).

## Frontend changes (`web/src`)

- `ProjectReconciliationCard.tsx:512-553` `updateMaintainerStatuses` — already operates from
  a project-scoped card; just needs to send the project ID in the request body to the updated
  `/api/maintainers/status` endpoint. No UX change needed — this fixes the component to match
  what its own UI already implies.
- `ProjectRouteClient.tsx:574` — same: add `projectId` to the request payload.
- `MaintainerCard.tsx:286` — the `Emeritus`/etc. `<select>` here needs checking: if this is the
  maintainer-detail-page editor (not project-scoped), it should either move to a per-project
  control or be removed in favor of project-scoped editing only. Needs a look during
  implementation to see which context this card renders in.

## Testing plan

- Model/migration: unit test that backfill copies the pre-migration global value onto every
  existing `MaintainerProject` row for that maintainer.
- Store: unit test that `UpdateMaintainerProjectStatus` only changes the targeted
  `(maintainerID, projectID)` row and leaves the maintainer's other project links untouched —
  this is the direct regression test for the bug as reported.
- `cmd/sanitize`: test that processing Project A's roster does not change a maintainer's
  status on Project B.
- Web-bff: update existing handler tests for `handleFossaInvite`, `handleFossaTeamSync`,
  `handleServiceInvitation`, `handleMaintainerStatusUpdate` to assert project-scoped behavior.
- BDD: extend/add a `features/web/` scenario that sets a maintainer Emeritus on one project
  and asserts their status is unchanged on a second project they belong to.

## Open questions

- **Cross-project views (search, LFX enrichment sweep).** What should "status" mean when a
  maintainer is being considered independent of any one project? Options: show/require a
  per-project breakdown instead of a single value; treat "active on at least one project" as
  the summary; or drop status from these views entirely and let callers pass a project ID
  when it matters. Needs a product decision before touching `handleSearchMaintainers` and
  `lfx/enricher.go`.
- **CRD schema.** See above — needs a survey of `apis.Maintainer.Status` consumers before
  deciding between the three options.
- **`maintainers.maintainer_status` column removal timing.** Proposed as a follow-up
  migration after the cutover is confirmed stable; needs a concrete "stable for N days/weeks"
  criterion or an explicit owner sign-off rather than an open-ended "later."
- **Historical miscategorization.** The backfill (migration step 2) carries forward any
  already-wrong global values as the per-project starting point. Worth a one-time manual
  review pass of maintainers linked to more than one project after the migration, to catch
  cases where the bug already caused an incorrect status to leak across projects.

## Relationship to the AI-assisted identity reconciliation work

This fix is a prerequisite for `specifications/ai-assisted-identity-reconciliation.md`, not
parallel work: that design assumes reconciling maintainer state per project across sources,
which requires status to actually be a per-project fact in the DB. Land this on its own branch
(`fix/per-project-maintainer-status`, off `main`) and merge before resuming the
identity-reconciliation implementation, so that work isn't designed against data it already
knows is wrong.
