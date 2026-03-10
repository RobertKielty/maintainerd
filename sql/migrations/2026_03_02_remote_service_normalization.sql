-- Remote service normalization migration (DRAFT)
-- Goal: replace service_* tables with remote_* tables and make column names explicit.

-- =========================
-- PostgreSQL (preferred)
-- =========================
BEGIN;

-- Drop join table that depends on service_users.
DROP TABLE IF EXISTS foundation_officer_service_users;

-- Drop legacy tables (data loss acceptable).
DROP TABLE IF EXISTS service_user_teams;
DROP TABLE IF EXISTS service_users;
DROP TABLE IF EXISTS service_teams;

-- Create normalized tables.
CREATE TABLE IF NOT EXISTS remote_teams (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ,
  project_id BIGINT NOT NULL,
  service_id BIGINT NOT NULL,
  remote_team_id BIGINT NOT NULL,
  remote_team_name TEXT,
  project_name TEXT
);
CREATE INDEX IF NOT EXISTS idx_remote_teams_project_id ON remote_teams(project_id);
CREATE INDEX IF NOT EXISTS idx_remote_teams_service_id ON remote_teams(service_id);
CREATE INDEX IF NOT EXISTS idx_remote_teams_remote_team_id ON remote_teams(remote_team_id);

CREATE TABLE IF NOT EXISTS remote_users (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ,
  service_id BIGINT NOT NULL,
  remote_user_id BIGINT NOT NULL,
  service_email VARCHAR(254) NOT NULL DEFAULT 'EMAIL_MISSING',
  remote_ref TEXT,
  service_git_hub_name TEXT
);
CREATE INDEX IF NOT EXISTS idx_remote_users_service_id ON remote_users(service_id);
CREATE INDEX IF NOT EXISTS idx_remote_users_remote_user_id ON remote_users(remote_user_id);

CREATE TABLE IF NOT EXISTS remote_team_users (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ,
  service_id BIGINT NOT NULL,
  team_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  maintainer_id BIGINT,
  collaborator_id BIGINT
);
CREATE INDEX IF NOT EXISTS idx_remote_team_users_service_id ON remote_team_users(service_id);
CREATE INDEX IF NOT EXISTS idx_remote_team_users_team_id ON remote_team_users(team_id);
CREATE INDEX IF NOT EXISTS idx_remote_team_users_user_id ON remote_team_users(user_id);
CREATE INDEX IF NOT EXISTS idx_remote_team_users_maintainer_id ON remote_team_users(maintainer_id);

-- Recreate join table to remote_users.
CREATE TABLE IF NOT EXISTS foundation_officer_service_users (
  foundation_officer_id BIGINT NOT NULL,
  remote_user_id BIGINT NOT NULL,
  PRIMARY KEY (foundation_officer_id, remote_user_id)
);
CREATE INDEX IF NOT EXISTS idx_foundation_officer_service_users_officer ON foundation_officer_service_users(foundation_officer_id);
CREATE INDEX IF NOT EXISTS idx_foundation_officer_service_users_user ON foundation_officer_service_users(remote_user_id);

-- Rename service_invitations.service_team_id -> remote_team_id
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='service_invitations' AND column_name='service_team_id'
  ) THEN
    ALTER TABLE service_invitations RENAME COLUMN service_team_id TO remote_team_id;
  END IF;
END $$;

COMMIT;

-- =========================
-- SQLite (draft)
-- =========================
-- SQLite cannot rename/drop columns easily; use table rebuild.
-- 1) Create new tables remote_teams, remote_users, remote_team_users
-- 2) Copy data if desired (skipped here; data loss acceptable)
-- 3) Drop old service_* tables
-- 4) Rebuild service_invitations with remote_team_id column

-- Example (simplified):
-- CREATE TABLE remote_teams (...);
-- CREATE TABLE remote_users (...);
-- CREATE TABLE remote_team_users (...);
-- DROP TABLE service_user_teams;
-- DROP TABLE service_users;
-- DROP TABLE service_teams;
-- ALTER TABLE service_invitations RENAME TO service_invitations_old;
-- CREATE TABLE service_invitations (..., remote_team_id INTEGER NOT NULL, ...);
-- INSERT INTO service_invitations (...) SELECT ..., service_team_id, ... FROM service_invitations_old;
-- DROP TABLE service_invitations_old;
