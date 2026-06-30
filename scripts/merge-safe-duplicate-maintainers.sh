#!/usr/bin/env bash
set -euo pipefail

# Merges SAFE_MERGE duplicate maintainer groups (same person, corroborated) into
# the OLDEST maintainer record, preserving the full set of project memberships.
#
# SAFE_MERGE is detected conservatively against the production maintainers table:
# a group of records sharing the same normalized GitHub handle is merged only when
#   - all records resolve to the same token-sorted name key
#     ("Peng Tao" and "Tao Peng" match; "Haotao Geng" and "Jianbo Yan" do not),
#   - no two records carry different non-empty lfx_user_id values,
#   - every email is a known sentinel or structurally valid (rejects garbage like
#     salaboy's free-text email -> CORRUPT_DATA),
#   - the group's identity observations do not carry conflicting LFIDs
#     (rejects kingdonb, whose observations point to a different real person).
# Groups that fail any check are left untouched for human review.
#
# The canonical (surviving) record is the OLDEST in the group (created_at asc,
# then id asc). Foreign-key references on the duplicates
#   - maintainer_projects
#   - maintainer_identity_observations
#   - audit_logs
#   - remote_team_users
#   - service_invitations
# are reassigned to the canonical record, the duplicates are soft-deleted, and the
# canonical's project set is asserted equal to the pre-merge union (abort on drift).
#
# Dry-run by default: the whole transaction is rolled back and nothing is committed.
# Pass --apply to commit. Runs through scripts/psql-maintainerd.sh, so the same
# MD_DB_* environment (and MD_DB_PASSWORD) it requires applies here.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPLY=false
HANDLE=""
ENV="local"
WEB_BASE=""

usage() {
  cat <<'USAGE'
Usage: scripts/merge-safe-duplicate-maintainers.sh [--env local|prod] [--web-base <url>] [--apply] [--handle <normalized_github>]

  --env <target>       Which database to target (default: local).
                         local -> scripts/psql-local-maintainerd.sh (podman compose DB)
                         prod  -> scripts/psql-maintainerd.sh        (production DB)
                       Accepts --env=<target> or --env <target>.
  --web-base <url>     Base URL for maintainerd web UI links in the review report.
                       Defaults to http://localhost:3000 for --env local and
                       https://maintainer-d.cncf.io for --env prod.
                       Accepts --web-base=<url> or --web-base <url>.
  --apply              Commit the merges. Without it, the script runs as a dry-run
                       and rolls back (prints exactly what it would change).
  --handle <handle>    Restrict to a single normalized GitHub handle (lower-cased),
                       e.g. --handle robertkielty. Useful for a first verified run.
  -h, --help           Show this help.

Examples:
  scripts/merge-safe-duplicate-maintainers.sh                              # dry-run, local, all SAFE groups
  scripts/merge-safe-duplicate-maintainers.sh --handle robertkielty        # dry-run, local, one group
  scripts/merge-safe-duplicate-maintainers.sh --env prod --handle robertkielty --apply
  scripts/merge-safe-duplicate-maintainers.sh --env prod --web-base https://maintainer-d.cncf.io
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apply)
      APPLY=true
      shift
      ;;
    --handle)
      if [[ $# -lt 2 ]]; then
        echo "--handle requires a value" >&2
        usage >&2
        exit 1
      fi
      HANDLE="$2"
      shift 2
      ;;
    --web-base)
      if [[ $# -lt 2 ]]; then
        echo "--web-base requires a value" >&2
        usage >&2
        exit 1
      fi
      WEB_BASE="$2"
      shift 2
      ;;
    --web-base=*)
      WEB_BASE="${1#--web-base=}"
      shift
      ;;
    --env)
      if [[ $# -lt 2 ]]; then
        echo "--env requires a value (local|prod)" >&2
        usage >&2
        exit 1
      fi
      ENV="$2"
      shift 2
      ;;
    --env=*)
      ENV="${1#--env=}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

case "$ENV" in
  local)
    PSQL="${SCRIPT_DIR}/psql-local-maintainerd.sh"
    [[ -z "$WEB_BASE" ]] && WEB_BASE="http://localhost:3000"
    ;;
  prod|production)
    PSQL="${SCRIPT_DIR}/psql-maintainerd.sh"
    [[ -z "$WEB_BASE" ]] && WEB_BASE="https://maintainer-d.cncf.io"
    ;;
  *)
    echo "Invalid --env value: ${ENV} (expected local or prod)" >&2
    usage >&2
    exit 1
    ;;
esac

if [[ ! -x "$PSQL" ]]; then
  echo "Required helper not found or not executable: $PSQL" >&2
  exit 1
fi

echo "Target environment: ${ENV} (${PSQL})" >&2
echo "Web base URL: ${WEB_BASE}" >&2

SQL_FILE="$(mktemp -t merge-safe-duplicate-maintainers.XXXXXX.sql)"
trap 'rm -f "$SQL_FILE"' EXIT
echo "SQL script: ${SQL_FILE}" >&2

cat > "$SQL_FILE" <<'SQL'
\set ON_ERROR_STOP on

-- Defaults if invoked directly via psql without -v.
\if :{?apply}
\else
  \set apply false
\endif
\if :{?handle}
\else
  \set handle ''
\endif
\if :{?web_base}
\else
  \set web_base 'http://localhost:3000'
\endif

\echo '============================================================'
\echo 'SAFE_MERGE duplicate maintainers'
\echo 'apply mode =' :apply
\echo 'handle filter =' :'handle'
\echo 'web base URL =' :'web_base'
\echo '============================================================'

begin;

-- Per-maintainer rows for every active, real-handle maintainer that belongs to a
-- duplicate normalized-GitHub group, with a token-sorted name key for comparison.
create temporary table dup_base on commit drop as
with base as (
  select
    m.id,
    m.name,
    m.email,
    m.git_hub_email,
    m.maintainer_status,
    m.created_at,
    lower(btrim(m.git_hub_account)) as norm_gh,
    nullif(btrim(m.lfx_user_id), '') as lfx,
    (
      select string_agg(t, ' ' order by t)
      from regexp_split_to_table(
             lower(regexp_replace(coalesce(m.name, ''), '[^a-z0-9]+', ' ', 'g')),
             '\s+'
           ) as t
      where t <> ''
    ) as name_key
  from maintainers m
  where m.deleted_at is null
    and coalesce(btrim(m.git_hub_account), '') <> ''
    and lower(btrim(m.git_hub_account)) <> 'github_missing'
)
select b.*
from base b
join (
  select norm_gh from base group by norm_gh having count(*) > 1
) dups on dups.norm_gh = b.norm_gh;

-- Classify every duplicate group. SAFE groups are merged; everything else is left
-- untouched and reported with the reason it was held back.
create temporary table dup_group_stats on commit drop as
with obs as (
  select
    db.norm_gh,
    count(distinct lower(btrim(o.lf_id)))
      filter (where coalesce(btrim(o.lf_id), '') <> '') as distinct_lfids
  from dup_base db
  join maintainer_identity_observations o
    on o.maintainer_id = db.id
   and o.deleted_at is null
  group by db.norm_gh
),
group_stats as (
  select
    db.norm_gh,
    count(*)                          as rec_count,
    count(distinct db.name_key)       as distinct_names,
    bool_and(db.name_key is not null and db.name_key <> '') as all_names_present,
    count(distinct db.lfx)            as distinct_lfx,
    bool_and(
      db.email in ('EMAIL_MISSING', 'GITHUB_MISSING', 'GITHUB_EMAIL_MISSING')
      or position('@' in coalesce(db.email, '')) > 1
    )                                 as all_emails_ok
  from dup_base db
  group by db.norm_gh
)
select
  gs.norm_gh,
  gs.rec_count,
  coalesce(obs.distinct_lfids, 0) as distinct_lfids,
  case
    when not gs.all_names_present              then 'REVIEW: empty or missing name'
    when gs.distinct_names > 1                 then 'REVIEW: name mismatch'
    when not gs.all_emails_ok                  then 'CORRUPT: non-identity email'
    when gs.distinct_lfx > 1                   then 'REVIEW: conflicting lfx_user_id'
    when coalesce(obs.distinct_lfids, 0) > 1   then 'REVIEW: conflicting observation LFID'
    else 'SAFE'
  end as classification
from group_stats gs
left join obs on obs.norm_gh = gs.norm_gh;

-- Members of every SAFE_MERGE group, each tagged with the canonical (oldest) id.
create temporary table safe_members on commit drop as
select
  db.norm_gh,
  db.id as maintainer_id,
  db.name,
  db.email,
  db.maintainer_status,
  db.created_at,
  first_value(db.id) over (
    partition by db.norm_gh
    order by db.created_at asc, db.id asc
  ) as canonical_id
from dup_base db
join dup_group_stats s
  on s.norm_gh = db.norm_gh
 and s.classification = 'SAFE'
where (:'handle' = '' or db.norm_gh = :'handle');

create temporary table merge_audit (
  norm_gh                    text,
  canonical_id               bigint,
  canonical_status           text,
  duplicate_ids              text,
  projects_before            text,
  projects_after             text,
  project_count_before       int,
  project_count_after        int,
  observations_moved         int,
  audit_logs_moved           int,
  remote_team_users_moved    int,
  service_invitations_moved  int,
  maintainers_soft_deleted   int
) on commit drop;

do $$
declare
  g            record;
  v_dups       bigint[];
  v_status     text;
  v_before     bigint[];
  v_after      bigint[];
  v_obs        int;
  v_audit      int;
  v_rtu        int;
  v_inv        int;
  v_del        int;
begin
  for g in
    select norm_gh, canonical_id
    from safe_members
    group by norm_gh, canonical_id
    order by norm_gh
  loop
    select array_agg(maintainer_id order by maintainer_id)
      into v_dups
      from safe_members
     where norm_gh = g.norm_gh
       and maintainer_id <> g.canonical_id;

    select maintainer_status
      into v_status
      from safe_members
     where maintainer_id = g.canonical_id
     limit 1;

    -- Project set before = union of memberships across every record in the group.
    select array_agg(distinct mp.project_id order by mp.project_id)
      into v_before
      from maintainer_projects mp
      join safe_members sm on sm.maintainer_id = mp.maintainer_id
     where sm.norm_gh = g.norm_gh;

    -- Move project memberships: add the canonical's missing ones, then drop the
    -- duplicates' rows. The composite PK on (maintainer_id, project_id) means a
    -- plain UPDATE would collide where both already share a project, so insert
    -- on-conflict-do-nothing and delete instead.
    insert into maintainer_projects (maintainer_id, project_id, joined_at)
    select g.canonical_id, mp.project_id, min(mp.joined_at)
      from maintainer_projects mp
     where mp.maintainer_id = any(v_dups)
     group by mp.project_id
    on conflict (maintainer_id, project_id) do nothing;

    delete from maintainer_projects
     where maintainer_id = any(v_dups);

    update maintainer_identity_observations
       set maintainer_id = g.canonical_id
     where maintainer_id = any(v_dups);
    get diagnostics v_obs = row_count;

    update audit_logs
       set maintainer_id = g.canonical_id
     where maintainer_id = any(v_dups);
    get diagnostics v_audit = row_count;

    update remote_team_users
       set maintainer_id = g.canonical_id
     where maintainer_id = any(v_dups);
    get diagnostics v_rtu = row_count;

    update service_invitations
       set maintainer_id = g.canonical_id
     where maintainer_id = any(v_dups);
    get diagnostics v_inv = row_count;

    update maintainers
       set deleted_at = now(),
           updated_at = now()
     where id = any(v_dups)
       and deleted_at is null;
    get diagnostics v_del = row_count;

    -- Project set after = memberships now held by the canonical alone.
    select array_agg(distinct mp.project_id order by mp.project_id)
      into v_after
      from maintainer_projects mp
     where mp.maintainer_id = g.canonical_id;

    if coalesce(v_before, '{}'::bigint[]) is distinct from coalesce(v_after, '{}'::bigint[]) then
      raise exception
        'project set changed for handle % (canonical %): before=% after=%',
        g.norm_gh, g.canonical_id, v_before, v_after;
    end if;

    insert into merge_audit values (
      g.norm_gh,
      g.canonical_id,
      v_status,
      array_to_string(v_dups, ' | '),
      array_to_string(v_before, ' | '),
      array_to_string(v_after, ' | '),
      coalesce(array_length(v_before, 1), 0),
      coalesce(array_length(v_after, 1), 0),
      v_obs, v_audit, v_rtu, v_inv, v_del
    );
  end loop;
end $$;

\echo ''
\echo '------------------------- MERGE REPORT -------------------------'
select * from merge_audit order by norm_gh;

\echo ''
\echo '------------------------- SUMMARY ------------------------------'
select
  count(*)                                       as groups_merged,
  coalesce(sum(maintainers_soft_deleted), 0)     as maintainers_soft_deleted,
  coalesce(sum(observations_moved), 0)           as observations_moved,
  coalesce(sum(audit_logs_moved), 0)             as audit_logs_moved,
  coalesce(sum(remote_team_users_moved), 0)      as remote_team_users_moved,
  coalesce(sum(service_invitations_moved), 0)    as service_invitations_moved
from merge_audit;

\echo ''
\echo '======== GROUPS NOT MERGED — human review required ==============='

-- Stash psql variables into session GUCs so the do-block can read them.
-- (psql :variables are not substituted inside $$ ... $$ blocks.)
\o /dev/null
select set_config('merge.handle_filter', :'handle',   true),
       set_config('merge.web_base',      :'web_base', true);
\o

-- Collect all review output here so it prints clean (no psql:file:line: NOTICE: prefix).
create temporary table review_report (
  seq  bigserial primary key,
  line text not null default ''
) on commit drop;

do $$
declare
  g    record;
  r    record;
  obs  record;
  proj record;
  v_handle   text := current_setting('merge.handle_filter', true);
  v_web      text := current_setting('merge.web_base',      true);
  v_proj_cnt int;
begin
  for g in
    select norm_gh, rec_count, distinct_lfids, classification
    from dup_group_stats
    where classification <> 'SAFE'
      and (v_handle = '' or norm_gh = v_handle)
    order by classification, norm_gh
  loop
    insert into review_report(line) values
      (''),
      ('----------------------------------------------------------------'),
      (format('HANDLE  : %s', g.norm_gh)),
      (format('REASON  : %s', g.classification)),
      (format('RECORDS : %s', g.rec_count));

    -- Per-record detail
    for r in
      select
        db.id,
        db.name,
        db.email,
        db.git_hub_email,
        db.lfx,
        db.maintainer_status,
        db.created_at::date as created
      from dup_base db
      where db.norm_gh = g.norm_gh
      order by db.created_at asc, db.id asc
    loop
      -- Count and list projects for this specific maintainer record
      select count(distinct mp.project_id)
        into v_proj_cnt
        from maintainer_projects mp
       where mp.maintainer_id = r.id;

      insert into review_report(line) values
        (format('  -- record id=%s  created=%s', r.id, r.created)),
        (format('     name         : %s', coalesce(r.name,          '<null>'))),
        (format('     email        : %s', coalesce(r.email,         '<null>'))),
        (format('     github email : %s', coalesce(r.git_hub_email, '<null>'))),
        (format('     lfx          : %s', coalesce(r.lfx,           '<null>'))),
        (format('     status       : %s', coalesce(r.maintainer_status, '<blank>'))),
        (format('     web          : %s/maintainers/%s', v_web, r.id)),
        (format('     projects (%s):', v_proj_cnt));

      -- Per-project references
      for proj in
        select
          p.id,
          p.name                                          as proj_name,
          nullif(btrim(p.maintainer_ref),       '')      as legacy_ref,
          nullif(btrim(p.dot_project_yaml_ref), '')      as yaml_ref,
          coalesce(st.maintainers_file_exists, false)    as yaml_exists
        from maintainer_projects mp
        join projects p             on p.id = mp.project_id and p.deleted_at is null
        left join dot_project_sync_states st on st.project_id = p.id
        where mp.maintainer_id = r.id
        order by p.name, p.id
      loop
        insert into review_report(line) values
          (format('       - %s', proj.proj_name)),
          (format('           route    : %s/projects/%s', v_web, proj.id)),
          (format('           legacy   : %s', coalesce(proj.legacy_ref, '<none>'))),
          (case
             when proj.yaml_exists and proj.yaml_ref is not null
               then format('           .project : %s', proj.yaml_ref)
             when proj.yaml_exists
               then          '           .project : maintainers.yaml exists (URL not stored)'
             else             '           .project : maintainers.yaml not found'
           end);
      end loop;
    end loop;

    -- Conflicting LFX observation detail
    if g.distinct_lfids > 1 then
      insert into review_report(line) values ('  -- conflicting identity observations:');
      for obs in
        select
          o.id              as obs_id,
          o.maintainer_id,
          o.lf_id,
          o.name            as obs_name,
          o.source,
          o.match_status,
          o.confidence,
          o.observed_at::date as observed
        from maintainer_identity_observations o
        join dup_base db on db.id = o.maintainer_id
        where db.norm_gh = g.norm_gh
          and coalesce(btrim(o.lf_id), '') <> ''
          and o.deleted_at is null
        order by o.maintainer_id, o.observed_at desc
      loop
        insert into review_report(line) values (format(
          '     obs id=%s  maintainer=%s  lf_id=%s  name=%s  source=%s  status=%s  confidence=%s  observed=%s',
          obs.obs_id, obs.maintainer_id, obs.lf_id, coalesce(obs.obs_name, ''),
          coalesce(obs.source, ''), coalesce(obs.match_status, ''),
          coalesce(obs.confidence, ''), obs.observed));
      end loop;
    end if;

    -- Guidance keyed to classification
    insert into review_report(line) values ('  GUIDANCE:');
    case
      when g.classification = 'REVIEW: name mismatch' then
        insert into review_report(line) values
          ('    Names do not token-sort to the same key — likely two different people'),
          ('    sharing this handle, or a name data entry error on one record.'),
          ('    Check each record via the web links above.'),
          ('    If same person: correct the name on the odd record, then re-run.'),
          ('    If different people: one record needs a corrected github_account.');
      when g.classification = 'REVIEW: conflicting lfx_user_id' then
        insert into review_report(line) values
          ('    Two records carry different non-empty lfx_user_id values.'),
          ('    Verify which LFX ID is correct for this GitHub handle via LFX.'),
          ('    Blank the wrong lfx_user_id, then re-run.');
      when g.classification = 'REVIEW: conflicting observation LFID' then
        insert into review_report(line) values
          ('    Identity observations reference more than one distinct LF ID (see above).'),
          ('    The observations may have been attached to the wrong maintainer record,'),
          ('    or the handle is shared by two real people.'),
          ('    Investigate via LFX, then either re-key the bad observation or'),
          ('    correct the github_account on the mis-matched record.');
      when g.classification = 'REVIEW: empty or missing name' then
        insert into review_report(line) values
          ('    At least one record has a blank/null name field.'),
          ('    Add a name to the incomplete record and re-run.');
      when g.classification like 'CORRUPT%' then
        insert into review_report(line) values
          ('    An identity field holds non-identity freeform text (not a sentinel).'),
          ('    Identify the garbage value above and either replace it with the correct'),
          ('    value or a recognised sentinel (EMAIL_MISSING / GITHUB_MISSING /'),
          ('    GITHUB_EMAIL_MISSING), then re-run.');
      else
        insert into review_report(line) values
          ('    Inspect the records above and resolve the classification reason.');
    end case;

  end loop;
end $$;

\set QUIET on
\pset format unaligned
\pset tuples_only on
\set QUIET off
select line from review_report order by seq;
\set QUIET on
\pset tuples_only off
\pset format aligned
\set QUIET off

\if :apply
  commit;
  \echo ''
  \echo '>>> APPLIED: changes committed.'
\else
  rollback;
  \echo ''
  \echo '>>> DRY-RUN: rolled back, nothing committed. Re-run with --apply to commit.'
\endif
SQL

exec "$PSQL" -v "apply=${APPLY}" -v "handle=${HANDLE}" -v "web_base=${WEB_BASE}" -f "$SQL_FILE"
