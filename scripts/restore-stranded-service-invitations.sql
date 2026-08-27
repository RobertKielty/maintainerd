\set ON_ERROR_STOP on

\if :{?apply}
\else
\set apply false
\endif

\echo 'maintainer-d service_invitations remediation'
\echo 'mode: apply=' :apply
\echo 'dry-run is the default; run with -v apply=true to update rows'
\echo ''
\echo 'Scope: a hand-curated, FOSSA-UI-verified list of invitation rows whose local'
\echo 'status/deleted_at no longer reflects the maintainer''s real FOSSA team membership.'
\echo 'This is NOT a heuristic sweep -- every row below was confirmed against FOSSA'
\echo 'team member lists / audit log by a human before this script was written.'
\echo 'See specifications/current-manual-maintainer-reconciliation-runbook.md and the'
\echo 'post-decommission-cleanup Item 3 investigation for how this list was derived.'
\echo ''
\echo 'No maintainer PII (emails/names) is stored in this script. The drift guard'
\echo 'below verifies each row''s service_email against an md5 hash captured at'
\echo 'curation time, rather than storing the plaintext address.'

DROP TABLE IF EXISTS si_remediation_target;

-- Each row here is one invitation whose corresponding person was independently
-- confirmed (via the FOSSA UI or FOSSA audit log) to be a real, current team member
-- on the date noted, even though the local row said otherwise. expected_project_id
-- and expected_email_md5 are asserted against the live row below as a staleness
-- guard -- if either has drifted since this list was curated, the script refuses
-- to run.
CREATE TEMP TABLE si_remediation_target (
  id, expected_project_id, expected_email_md5, note
) AS
VALUES
  (14,  244, '48305f2ba4822cc64f678ee110f67a83', 'confirmed FOSSA team member; row stranded pending/soft-deleted since 2026-03-03'),
  (45,  44,  'd830d4bfcd0f6cf1a99373ac9ecd4596', 'confirmed FOSSA team member; row stranded error/soft-deleted since 2026-03-06 (role-assignment API error)'),
  (204, 47,  'ccf94005d632ed48826dae989d964cdd', 'confirmed FOSSA team member; row stranded pending/soft-deleted since 2026-05-20'),
  (61,  140, '404581ebaad332b3442bed1d54aeb293', 'confirmed FOSSA team member; row stranded error/soft-deleted since 2026-03-11 (role-assignment API error)'),
  (264, 257, 'af105e719cafa17c871297369d068c0c', 'confirmed FOSSA team member (added manually via FOSSA UI); row stranded pending/soft-deleted since 2026-08-24'),
  (210, 259, '44a1d7d1288070de3002f509340230da', 'confirmed FOSSA team member; row stranded pending/soft-deleted since 2026-05-21'),
  (259, 257, 'd2e96044ce0e9e74cbe8cc7692b4e1f9', 'confirmed FOSSA team member (FOSSA_POLLER auto-add, audit log 2026-08-24); status later clobbered to error by a transient FetchUserInvitations 502, then soft-deleted by the handleFossaInviteRefresh guard bug'),
  (258, 257, '1d95515b436bed411141b61e1399fe99', 'confirmed FOSSA team member (FOSSA_POLLER auto-add, audit log 2026-08-25); accepted+done row soft-deleted by the handleFossaInviteRefresh guard bug'),
  (211, 259, 'b2be14a5fd6f2ae24abb2449dcaf40df', 'confirmed FOSSA team member; accepted+done row soft-deleted by the handleFossaInviteRefresh guard bug since 2026-05-22'),
  (208, 259, 'a09d3f3903e6127573f3025e842a95ad', 'confirmed FOSSA team member; status clobbered to error despite done team assignment (never soft-deleted, but visibly wrong today)'),
  (206, 259, '85d076ce208dabe2d1aee17eee15b103', 'confirmed FOSSA team member; status clobbered to error despite done team assignment (never soft-deleted, but visibly wrong today)');

-- Guard 1: the curated list must be exactly this size. If someone edits the VALUES
-- above without updating this number, the script refuses to run rather than silently
-- remediating a different-sized set than was reviewed.
DO $$
DECLARE
  target_count integer;
BEGIN
  SELECT count(*) INTO target_count FROM si_remediation_target;
  IF target_count <> 11 THEN
    RAISE EXCEPTION 'expected exactly 11 curated target rows, found % -- this script is scoped to a specific hand-verified list; update this guard only after re-verifying the new list against FOSSA', target_count;
  END IF;
END $$;

-- Guard 2: every target id must still exist.
DO $$
DECLARE
  missing_count integer;
BEGIN
  SELECT count(*) INTO missing_count
  FROM si_remediation_target t
  LEFT JOIN service_invitations si ON si.id = t.id
  WHERE si.id IS NULL;
  IF missing_count > 0 THEN
    RAISE EXCEPTION 'preflight failed: % target invitation id(s) no longer exist -- re-run the survey before proceeding', missing_count;
  END IF;
END $$;

-- Guard 3: every target row's project_id/service_email must still match what was
-- verified against FOSSA. service_email is compared as an md5 hash so no plaintext
-- address is stored in this file. If a row changed shape since the survey, don't
-- touch it blind.
DO $$
DECLARE
  mismatch_count integer;
BEGIN
  SELECT count(*) INTO mismatch_count
  FROM si_remediation_target t
  JOIN service_invitations si ON si.id = t.id
  WHERE si.project_id <> t.expected_project_id
     OR md5(lower(trim(si.service_email))) <> t.expected_email_md5;
  IF mismatch_count > 0 THEN
    RAISE EXCEPTION 'preflight failed: % target row(s) no longer match the expected project_id/service_email -- re-survey before proceeding', mismatch_count;
  END IF;
END $$;

-- Guard 4: idx_service_invite_project_email is a PLAIN unique index on
-- (service_id, project_id, service_email) -- it is NOT partial and does NOT exclude
-- soft-deleted rows. Restoring deleted_at=NULL on a target row would violate that
-- constraint if any other row (deleted or not) already occupies the same triple with
-- deleted_at IS NULL. Refuse to run if that's the case, rather than let Postgres
-- reject the UPDATE mid-script.
DO $$
DECLARE
  collision_count integer;
BEGIN
  SELECT count(*) INTO collision_count
  FROM si_remediation_target t
  JOIN service_invitations si ON si.id = t.id
  JOIN service_invitations other
    ON other.service_id = si.service_id
   AND other.project_id = si.project_id
   AND lower(trim(other.service_email)) = lower(trim(si.service_email))
   AND other.id <> si.id
   AND other.deleted_at IS NULL
  WHERE si.deleted_at IS NOT NULL;
  IF collision_count > 0 THEN
    RAISE EXCEPTION 'preflight failed: % target row(s) would collide with an existing live row on (service_id, project_id, service_email) after restore -- investigate the duplicate before running this script', collision_count;
  END IF;
END $$;

\echo ''
\echo 'preview: rows that would be restored/corrected (service_email withheld -- see production DB for the live value)'
SELECT
  si.id,
  si.project_id,
  si.status               AS current_status,
  si.team_assignment_status AS current_team_assignment_status,
  si.deleted_at            AS current_deleted_at,
  t.note
FROM si_remediation_target t
JOIN service_invitations si ON si.id = t.id
ORDER BY si.project_id, si.id;

\if :apply

\echo ''
\echo 'applying remediation'

BEGIN;

DROP TABLE IF EXISTS si_remediation_updated;

CREATE TEMP TABLE si_remediation_updated AS
WITH updated AS (
  UPDATE service_invitations si
  SET
    deleted_at             = NULL,
    status                 = 'accepted',
    team_assignment_status = 'done',
    team_add_attempts      = 0,
    next_team_add_at       = NULL,
    last_error             = NULL,
    last_checked_at        = now(),
    updated_at             = now()
  FROM si_remediation_target t
  WHERE si.id = t.id
    AND si.id IN (SELECT id FROM si_remediation_target)
  RETURNING si.id, si.project_id, si.maintainer_id, si.service_id
)
SELECT * FROM updated;

INSERT INTO audit_logs (created_at, updated_at, project_id, maintainer_id, service_id, action, message, metadata)
SELECT
  now(),
  now(),
  u.project_id,
  u.maintainer_id,
  u.service_id,
  'FOSSA_INVITATION_MANUAL_REMEDIATION',
  format(
    'Manually restored service_invitations id=%s to status=accepted/team_assignment_status=done after confirming real FOSSA team membership out-of-band; the row had been incorrectly soft-deleted or had its status clobbered by the handleFossaInviteRefresh guard bug (cmd/web-bff/main.go).',
    u.id
  ),
  json_build_object('invitation_id', u.id)::text
FROM si_remediation_updated u;

SELECT 'invitations_restored' AS metric, count(*) AS value FROM si_remediation_updated;

COMMIT;

\else

\echo ''
\echo 'dry run: no rows updated'
SELECT 'invitations_would_restore' AS metric, count(*) AS value FROM si_remediation_target;

\endif

\echo ''
\echo 'current state of targeted rows (post-apply or dry-run; service_email withheld)'
SELECT
  si.id,
  si.project_id,
  si.status,
  si.team_assignment_status,
  si.deleted_at,
  si.updated_at
FROM si_remediation_target t
JOIN service_invitations si ON si.id = t.id
ORDER BY si.project_id, si.id;
