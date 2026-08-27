package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"maintainerd/db"
	"maintainerd/geo"
	"maintainerd/model"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const (
	defaultDBPath      = "/data/maintainers.db"
	missingPlaceholder = "GITHUB_MISSING"
	// pauseBetweenRequests keeps us well under GitHub's 5000 req/hr authenticated limit.
	pauseBetweenRequests = 750 * time.Millisecond
)

func main() {
	ctx := context.Background()

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

	githubToken := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	if githubToken == "" {
		log.Fatal("GITHUB_API_TOKEN is required")
	}

	dbConn, err := db.OpenGorm(dbDriver, dsn, &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	var maintainers []model.Maintainer
	if err := dbConn.
		Where("git_hub_account != ? AND git_hub_account != '' AND deleted_at IS NULL", missingPlaceholder).
		Find(&maintainers).Error; err != nil {
		log.Fatalf("failed to load maintainers: %v", err)
	}

	log.Printf("github-profile-sync: syncing %d maintainers", len(maintainers))
	updated := 0
	skipped := 0
	errored := 0

	for _, m := range maintainers {
		ghUser, _, err := client.Users.Get(ctx, m.GitHubAccount)
		if err != nil {
			log.Printf("github-profile-sync: WARN failed to fetch %q: %v", m.GitHubAccount, err)
			errored++
			time.Sleep(pauseBetweenRequests)
			continue
		}

		rawLocation := strings.TrimSpace(ghUser.GetLocation())
		if rawLocation == "" {
			skipped++
			time.Sleep(pauseBetweenRequests)
			continue
		}

		updates := map[string]interface{}{
			"location": rawLocation,
			"country":  nil,
			"timezone": nil,
		}
		if country, tz, ok := geo.DeriveCountryAndTimezone(rawLocation); ok {
			updates["country"] = country
			updates["timezone"] = tz
		}

		if err := dbConn.Model(&model.Maintainer{}).
			Where("id = ?", m.ID).
			Updates(updates).Error; err != nil {
			log.Printf("github-profile-sync: WARN failed to update maintainer id=%d: %v", m.ID, err)
			errored++
		} else {
			updated++
		}

		time.Sleep(pauseBetweenRequests)
	}

	log.Printf("github-profile-sync: done — updated=%d skipped=%d errored=%d", updated, skipped, errored)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
