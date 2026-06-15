\set ON_ERROR_STOP on

\if :{?apply}
\else
\set apply false
\endif

\echo 'maintainer-d git_hub_org backfill'
\echo 'mode: apply=' :apply
\echo 'dry-run is the default; run with -v apply=true to update rows'

DROP TABLE IF EXISTS missing_git_hub_org_candidates;

CREATE TEMP TABLE missing_git_hub_org_candidates AS
SELECT
  p.id AS project_id,
  p.name AS project_name,
  inferred.source_column,
  inferred.source_url,
  inferred.git_hub_org AS inferred_git_hub_org
FROM projects p
CROSS JOIN LATERAL (
  SELECT
    source_column,
    source_url,
    git_hub_org
  FROM (
    SELECT
      refs.precedence,
      refs.source_column,
      refs.source_url,
      COALESCE(
        substring(refs.source_url FROM '^https?://(?:www\.)?github\.com/orgs/([^/?#]+)(?:[/?#]|$)'),
        substring(refs.source_url FROM '^https?://(?:www\.)?github\.com/(?!orgs(?:[/?#]|$))([^/?#]+)(?:[/?#]|$)'),
        substring(refs.source_url FROM '^https?://raw\.githubusercontent\.com/([^/?#]+)(?:[/?#]|$)')
      ) AS git_hub_org
    FROM (
      VALUES
        (1, 'dot_project_yaml_ref', p.dot_project_yaml_ref),
        (2, 'dot_project_repo_ref', p.dot_project_repo_ref),
        (3, 'maintainer_ref', p.maintainer_ref)
    ) AS refs(precedence, source_column, source_url)
    WHERE NULLIF(trim(refs.source_url), '') IS NOT NULL
  ) extracted
  WHERE git_hub_org IS NOT NULL
  ORDER BY precedence
  LIMIT 1
) inferred
WHERE p.deleted_at IS NULL
  AND NULLIF(trim(p.git_hub_org), '') IS NULL;

DROP TABLE IF EXISTS missing_git_hub_org_unresolved;

CREATE TEMP TABLE missing_git_hub_org_unresolved AS
SELECT
  p.id AS project_id,
  p.name AS project_name,
  p.dot_project_yaml_ref,
  p.dot_project_repo_ref,
  p.maintainer_ref
FROM projects p
WHERE p.deleted_at IS NULL
  AND NULLIF(trim(p.git_hub_org), '') IS NULL
  AND NOT EXISTS (
    SELECT 1
    FROM missing_git_hub_org_candidates c
    WHERE c.project_id = p.id
  );

\echo ''
\echo 'preview: rows that would be backfilled'
SELECT
  project_id,
  project_name,
  inferred_git_hub_org,
  source_column,
  source_url
FROM missing_git_hub_org_candidates
ORDER BY lower(project_name), project_id
LIMIT 100;

\echo ''
\echo 'metrics before update'
SELECT 'projects_missing_git_hub_org' AS metric, count(*) AS value
FROM projects
WHERE deleted_at IS NULL
  AND NULLIF(trim(git_hub_org), '') IS NULL
UNION ALL
SELECT 'projects_backfillable_from_refs', count(*)
FROM missing_git_hub_org_candidates
UNION ALL
SELECT 'projects_remaining_after_backfill', count(*)
FROM missing_git_hub_org_unresolved;

\if :apply

\echo ''
\echo 'applying git_hub_org backfill'

DROP TABLE IF EXISTS missing_git_hub_org_updated;

CREATE TEMP TABLE missing_git_hub_org_updated AS
WITH updated AS (
  UPDATE projects p
  SET
    git_hub_org = c.inferred_git_hub_org,
    updated_at = now()
  FROM missing_git_hub_org_candidates c
  WHERE p.id = c.project_id
    AND p.deleted_at IS NULL
    AND NULLIF(trim(p.git_hub_org), '') IS NULL
  RETURNING
    p.id AS project_id,
    p.name AS project_name,
    p.git_hub_org,
    c.source_column,
    c.source_url
)
SELECT * FROM updated;

SELECT 'projects_updated' AS metric, count(*) AS value
FROM missing_git_hub_org_updated;

SELECT
  project_id,
  project_name,
  git_hub_org,
  source_column,
  source_url
FROM missing_git_hub_org_updated
ORDER BY lower(project_name), project_id
LIMIT 100;

\else

\echo ''
\echo 'dry run: no rows updated'
SELECT 'projects_would_update' AS metric, count(*) AS value
FROM missing_git_hub_org_candidates;

\endif

\echo ''
\echo 'metrics after update or dry-run'
SELECT 'projects_missing_git_hub_org' AS metric, count(*) AS value
FROM projects
WHERE deleted_at IS NULL
  AND NULLIF(trim(git_hub_org), '') IS NULL
UNION ALL
SELECT 'projects_unresolved_after_prospective_backfill', count(*)
FROM missing_git_hub_org_unresolved;

\echo ''
\echo 'remaining unresolved projects without git_hub_org after prospective backfill'
SELECT
  project_id,
  project_name,
  dot_project_yaml_ref,
  dot_project_repo_ref,
  maintainer_ref
FROM missing_git_hub_org_unresolved
ORDER BY lower(project_name), project_id
LIMIT 100;
