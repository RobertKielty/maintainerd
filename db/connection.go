package db

import (
	"fmt"

	"maintainerd/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// OpenGorm opens a gorm DB connection using the specified driver and DSN.
// driver: "sqlite" or "postgres"
// dsn: sqlite file path or Postgres DSN.
func OpenGorm(driver, dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	var conn *gorm.DB
	var err error
	switch driver {
	case "", "sqlite":
		if dsn == "" {
			return nil, fmt.Errorf("sqlite requires a db path")
		}
		conn, err = gorm.Open(sqlite.Open(dsn), cfg)
	case "postgres":
		if dsn == "" {
			return nil, fmt.Errorf("postgres requires a DSN")
		}
		conn, err = gorm.Open(postgres.Open(dsn), cfg)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", driver)
	}
	if err != nil {
		return nil, err
	}

	// Maintainer.Projects and Project.Maintainers both declare the many2many
	// "maintainer_projects" relation. Without pinning MaintainerProject as the
	// explicit join model on both sides, GORM parses two separate *implicit* join
	// schemas (2 FK columns each, one per relation) alongside the real
	// MaintainerProject schema. AutoMigrate.ReorderModels keys its work list by table
	// name, so whichever of these same-named schemas is processed last silently wins
	// - and that can be an implicit one, dropping columns like Status. Registering the
	// real join model on both sides collapses them to a single schema.
	if err := conn.SetupJoinTable(&model.Maintainer{}, "Projects", &model.MaintainerProject{}); err != nil {
		return nil, fmt.Errorf("setup join table maintainer->projects: %w", err)
	}
	if err := conn.SetupJoinTable(&model.Project{}, "Maintainers", &model.MaintainerProject{}); err != nil {
		return nil, fmt.Errorf("setup join table project->maintainers: %w", err)
	}

	return conn, nil
}
