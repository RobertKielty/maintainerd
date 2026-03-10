BEGIN;

ALTER TABLE service_invitations
  ADD COLUMN IF NOT EXISTS team_assignment_status TEXT;

ALTER TABLE service_invitations
  ADD COLUMN IF NOT EXISTS team_add_attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE service_invitations
  ADD COLUMN IF NOT EXISTS next_team_add_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_service_invitations_team_assignment_status
  ON service_invitations (team_assignment_status);

CREATE INDEX IF NOT EXISTS idx_service_invitations_next_team_add_at
  ON service_invitations (next_team_add_at);

COMMIT;
