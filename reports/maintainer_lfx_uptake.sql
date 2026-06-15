
  WITH latest_lfx AS (
    SELECT DISTINCT ON (maintainer_id)
      maintainer_id,
      source_user_id,
      lf_id,
      company_name,
      observed_at
    FROM maintainer_identity_observations
    WHERE source = 'lfx'
      AND match_status = 'matched'
      AND maintainer_id IS NOT NULL
      AND deleted_at IS NULL
    ORDER BY maintainer_id, observed_at DESC, id DESC
  )
  SELECT
    p.name AS project_name,
    m.name AS real_name,
    m.git_hub_account,
    COALESCE(NULLIF(latest_lfx.lf_id, ''), NULLIF(m.lfx_user_id, ''),
    NULLIF(latest_lfx.source_user_id, '')) AS lfid,
    COALESCE(NULLIF(c.name, ''), NULLIF(latest_lfx.company_name, ''))
    AS company_name
  FROM maintainers m
  JOIN maintainer_projects mp
    ON mp.maintainer_id = m.id
  JOIN projects p
    ON p.id = mp.project_id
  LEFT JOIN companies c
    ON c.id = m.company_id
  LEFT JOIN latest_lfx
    ON latest_lfx.maintainer_id = m.id
  WHERE m.deleted_at IS NULL
    AND p.deleted_at IS NULL
    AND (
      NULLIF(m.lfx_user_id, '') IS NOT NULL
      OR latest_lfx.maintainer_id IS NOT NULL
    )
  ORDER BY
    p.name,
    m.name,
    m.git_hub_account;
