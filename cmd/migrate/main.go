package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"gorm.io/gorm"
)

const defaultDBPath = "/data/maintainers.db"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dbDriver := envOr("MD_DB_DRIVER", "sqlite")
	dbDSN := envOr("MD_DB_DSN", "")
	dbPath := envOr("MD_DB_PATH", defaultDBPath)
	if dbDriver == "postgres" && dbDSN == "" {
		log.Fatal("MD_DB_DSN is required when MD_DB_DRIVER=postgres")
	}
	dsn := dbPath
	if dbDriver == "postgres" {
		dsn = dbDSN
	}

	dbConn, err := db.OpenGorm(dbDriver, dsn, &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	store := db.NewSQLStore(dbConn)

	if err := migrate(ctx, store); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}

	log.Println("migration complete")
}

func migrate(_ context.Context, store *db.SQLStore) error {
	if err := store.DB().AutoMigrate(
		&model.AuditLog{},
		&model.Collaborator{},
		&model.Company{},
		&model.Foundation{},
		&model.FoundationOfficer{},
		&model.StaffMember{},
		&model.Maintainer{},
		&model.MaintainerProject{},
		&model.Project{},
		&model.Service{},
		&model.RemoteTeam{},
		&model.RemoteUser{},
		&model.RemoteTeamUser{},
		&model.ServiceInvitation{},
		&model.MaintainerRefCache{},
	); err != nil {
		return err
	}

	if store.DB().Name() != "postgres" {
		return nil
	}

	extensions := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE EXTENSION IF NOT EXISTS unaccent",
		`CREATE OR REPLACE FUNCTION unaccent_immutable(text)
		 RETURNS text
		 LANGUAGE sql
		 IMMUTABLE
		 PARALLEL SAFE
		 AS $$ SELECT unaccent($1); $$`,
	}
	for _, stmt := range extensions {
		if err := store.DB().Exec(stmt).Error; err != nil {
			return err
		}
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_projects_name_trgm ON projects USING gin (lower(name) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_projects_ref_trgm ON projects USING gin (lower(maintainer_ref) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_projects_dotref_trgm ON projects USING gin (lower(dot_project_yaml_ref) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_projects_github_org_trgm ON projects USING gin (lower(git_hub_org) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_maintainers_name_trgm ON maintainers USING gin (lower(name) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_maintainers_email_trgm ON maintainers USING gin (lower(email) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_maintainers_github_trgm ON maintainers USING gin (lower(git_hub_account) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_companies_name_trgm ON companies USING gin (lower(name) gin_trgm_ops)",
		"ALTER TABLE maintainers ADD COLUMN IF NOT EXISTS search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', unaccent_immutable(coalesce(name, '') || ' ' || coalesce(email, '') || ' ' || coalesce(git_hub_account, '')))) STORED",
		"ALTER TABLE projects ADD COLUMN IF NOT EXISTS search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', unaccent_immutable(coalesce(name, '') || ' ' || coalesce(maintainer_ref, '') || ' ' || coalesce(dot_project_yaml_ref, '') || ' ' || coalesce(git_hub_org, '')))) STORED",
		"CREATE INDEX IF NOT EXISTS idx_maintainers_search_tsv ON maintainers USING gin (search_tsv)",
		"CREATE INDEX IF NOT EXISTS idx_projects_search_tsv ON projects USING gin (search_tsv)",
	}
	for _, stmt := range indexes {
		if err := store.DB().Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
