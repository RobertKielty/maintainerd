\set ON_ERROR_STOP on

\if :{?apply}
\else
\set apply false
\endif

\echo 'maintainer-d project delete'
\echo 'mode: apply=' :apply
\echo 'dry-run is the default; run with -v apply=true to delete the maintainer-d project'

DROP TABLE IF EXISTS maintainer_d_project_target;

CREATE TEMP TABLE maintainer_d_project_target AS
SELECT
  id AS project_id,
  name AS project_name,
  deleted_at
FROM projects
WHERE lower(trim(name)) = 'maintainer-d';

DO $$
DECLARE
  target_count integer;
BEGIN
  SELECT count(*) INTO target_count FROM maintainer_d_project_target;
  IF target_count > 1 THEN
    RAISE EXCEPTION 'refusing to delete: expected at most one maintainer-d project, found %', target_count;
  END IF;
END $$;

\echo ''
\echo 'target project'
SELECT
  project_id,
  project_name,
  deleted_at
FROM maintainer_d_project_target;

\echo ''
\echo 'preview: associated rows that would be removed or updated'
DROP TABLE IF EXISTS maintainer_d_project_relation_counts;

CREATE TEMP TABLE maintainer_d_project_relation_counts (
  relation text PRIMARY KEY,
  rows_affected bigint NOT NULL
);

DO $$
BEGIN
  IF to_regclass('public.audit_logs') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'audit_logs', count(*)
    FROM audit_logs
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('audit_logs', 0);
  END IF;

  IF to_regclass('public.dot_project_sync_states') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'dot_project_sync_states', count(*)
    FROM dot_project_sync_states
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('dot_project_sync_states', 0);
  END IF;

  IF to_regclass('public.maintainer_identity_observations') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'maintainer_identity_observations', count(*)
    FROM maintainer_identity_observations
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('maintainer_identity_observations', 0);
  END IF;

  IF to_regclass('public.maintainer_projects') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'maintainer_projects', count(*)
    FROM maintainer_projects
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('maintainer_projects', 0);
  END IF;

  IF to_regclass('public.maintainer_ref_caches') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'maintainer_ref_caches', count(*)
    FROM maintainer_ref_caches
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('maintainer_ref_caches', 0);
  END IF;

  IF to_regclass('public.reconciliation_results') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'reconciliation_results', count(*)
    FROM reconciliation_results
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('reconciliation_results', 0);
  END IF;

  IF to_regclass('public.remote_team_users') IS NOT NULL AND to_regclass('public.remote_teams') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'remote_team_users', count(*)
    FROM remote_team_users
    WHERE team_id IN (
      SELECT id
      FROM remote_teams
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
    );
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('remote_team_users', 0);
  END IF;

  IF to_regclass('public.remote_teams') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'remote_teams', count(*)
    FROM remote_teams
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('remote_teams', 0);
  END IF;

  IF to_regclass('public.service_invitations') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'service_invitations', count(*)
    FROM service_invitations
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('service_invitations', 0);
  END IF;

  IF to_regclass('public.service_projects') IS NOT NULL THEN
    INSERT INTO maintainer_d_project_relation_counts
    SELECT 'service_projects', count(*)
    FROM service_projects
    WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target);
  ELSE
    INSERT INTO maintainer_d_project_relation_counts VALUES ('service_projects', 0);
  END IF;

  INSERT INTO maintainer_d_project_relation_counts
  SELECT 'child_projects_parent_project_id_to_clear', count(*)
  FROM projects
  WHERE parent_project_id IN (SELECT project_id FROM maintainer_d_project_target);

  INSERT INTO maintainer_d_project_relation_counts
  SELECT 'projects', count(*)
  FROM maintainer_d_project_target;
END $$;

SELECT relation, rows_affected
FROM maintainer_d_project_relation_counts
ORDER BY relation;

\if :apply

\echo ''
\echo 'applying maintainer-d project delete'

BEGIN;

DROP TABLE IF EXISTS maintainer_d_project_deleted_counts;

CREATE TEMP TABLE maintainer_d_project_deleted_counts (
  relation text PRIMARY KEY,
  rows_affected bigint NOT NULL
);

DO $$
BEGIN
  IF to_regclass('public.audit_logs') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM audit_logs
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'audit_logs', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('audit_logs', 0);
  END IF;

  IF to_regclass('public.dot_project_sync_states') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM dot_project_sync_states
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'dot_project_sync_states', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('dot_project_sync_states', 0);
  END IF;

  IF to_regclass('public.maintainer_identity_observations') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM maintainer_identity_observations
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'maintainer_identity_observations', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('maintainer_identity_observations', 0);
  END IF;

  IF to_regclass('public.maintainer_projects') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM maintainer_projects
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'maintainer_projects', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('maintainer_projects', 0);
  END IF;

  IF to_regclass('public.maintainer_ref_caches') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM maintainer_ref_caches
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'maintainer_ref_caches', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('maintainer_ref_caches', 0);
  END IF;

  IF to_regclass('public.reconciliation_results') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM reconciliation_results
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'reconciliation_results', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('reconciliation_results', 0);
  END IF;

  IF to_regclass('public.remote_team_users') IS NOT NULL AND to_regclass('public.remote_teams') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM remote_team_users
      WHERE team_id IN (
        SELECT id
        FROM remote_teams
        WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      )
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'remote_team_users', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('remote_team_users', 0);
  END IF;

  IF to_regclass('public.remote_teams') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM remote_teams
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'remote_teams', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('remote_teams', 0);
  END IF;

  IF to_regclass('public.service_invitations') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM service_invitations
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'service_invitations', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('service_invitations', 0);
  END IF;

  IF to_regclass('public.service_projects') IS NOT NULL THEN
    WITH deleted AS (
      DELETE FROM service_projects
      WHERE project_id IN (SELECT project_id FROM maintainer_d_project_target)
      RETURNING 1
    )
    INSERT INTO maintainer_d_project_deleted_counts
    SELECT 'service_projects', count(*) FROM deleted;
  ELSE
    INSERT INTO maintainer_d_project_deleted_counts VALUES ('service_projects', 0);
  END IF;
END $$;

WITH updated AS (
  UPDATE projects
  SET
    parent_project_id = NULL,
    updated_at = now()
  WHERE parent_project_id IN (SELECT project_id FROM maintainer_d_project_target)
  RETURNING 1
)
INSERT INTO maintainer_d_project_deleted_counts
SELECT 'child_projects_parent_project_id_cleared', count(*) FROM updated;

WITH deleted AS (
  DELETE FROM projects
  WHERE id IN (SELECT project_id FROM maintainer_d_project_target)
  RETURNING 1
)
INSERT INTO maintainer_d_project_deleted_counts
SELECT 'projects', count(*) FROM deleted;

COMMIT;

\echo ''
\echo 'delete metrics'
SELECT relation, rows_affected
FROM maintainer_d_project_deleted_counts
ORDER BY relation;

\else

\echo ''
\echo 'dry run: no rows deleted'

\endif

\echo ''
\echo 'post-check: maintainer-d projects remaining'
SELECT
  id AS project_id,
  name AS project_name,
  deleted_at
FROM projects
WHERE lower(trim(name)) = 'maintainer-d'
ORDER BY id;
