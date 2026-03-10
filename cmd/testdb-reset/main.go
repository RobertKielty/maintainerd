package main

import (
	"log"
	"os"
	"strings"

	"maintainerd/db"

	"gorm.io/gorm"
)

const defaultDBPath = "/data/maintainers.db"

func main() {
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

	switch dbDriver {
	case "postgres":
		if err := dbConn.Exec("DROP SCHEMA IF EXISTS public CASCADE").Error; err != nil {
			log.Fatalf("failed to drop public schema: %v", err)
		}
		if err := dbConn.Exec("CREATE SCHEMA public").Error; err != nil {
			log.Fatalf("failed to create public schema: %v", err)
		}
	default:
		if dbPath == "" {
			log.Fatal("MD_DB_PATH is required when MD_DB_DRIVER=sqlite")
		}
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			log.Fatalf("failed to remove sqlite DB %s: %v", dbPath, err)
		}
	}

	log.Printf("test db reset complete (driver=%s)", dbDriver)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
