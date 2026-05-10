package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"maintainerd/db"
	"maintainerd/dotproject"
	"maintainerd/model"

	"github.com/google/go-github/v55/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const defaultDBPath = "/data/maintainers.db"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dbDriver := envOr("MD_DB_DRIVER", "sqlite")
	dbDSN := envOr("MD_DB_DSN", "")
	dbPath := envOr("MD_DB_PATH", defaultDBPath)
	if dbDriver == "postgres" && dbDSN == "" {
		log.Fatal("MD_DB_DSN is required when MD_DB_DRIVER=postgres")
	}

	githubToken := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	if githubToken == "" {
		log.Fatal("GITHUB_API_TOKEN is required")
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
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	syncer := &dotproject.Syncer{
		Store: store,
		Discoverer: &dotproject.Discoverer{
			Client: &dotproject.GitHubRepositoryClient{Client: client},
		},
	}

	summary, err := syncer.SyncAll(ctx)
	if err != nil {
		log.Fatalf("dot-project sync failed: %v", err)
	}

	logger := zap.NewNop().Sugar()
	if err := store.LogAuditEvent(logger, buildAuditEvent(summary)); err != nil {
		log.Printf("dot-project sync audit log failed: %v", err)
	}

	log.Printf(
		"dot-project sync complete loaded=%d total=%d skipped=%d skipped_archived=%d skipped_excluded=%d synced=%d errored=%d github_errors=%d rate_limit_errors=%d not_found=%d repo_only=%d partial=%d adopted=%d",
		summary.Loaded,
		summary.Total,
		summary.Skipped,
		summary.SkippedArchived,
		summary.SkippedExcluded,
		summary.Synced,
		summary.Errored,
		summary.GitHubErrorCount,
		summary.RateLimitErrorCount,
		summary.NotFound,
		summary.RepoOnly,
		summary.Partial,
		summary.Adopted,
	)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func buildAuditEvent(summary dotproject.SyncSummary) model.AuditLog {
	metadata := map[string]any{
		"loaded":                 summary.Loaded,
		"scanned":                summary.Total,
		"skipped":                summary.Skipped,
		"skipped_archived":       summary.SkippedArchived,
		"skipped_excluded":       summary.SkippedExcluded,
		"synced":                 summary.Synced,
		"errored":                summary.Errored,
		"github_error_count":     summary.GitHubErrorCount,
		"rate_limit_error_count": summary.RateLimitErrorCount,
		"not_found":              summary.NotFound,
		"repo_only":              summary.RepoOnly,
		"partial":                summary.Partial,
		"adopted":                summary.Adopted,
	}
	if len(summary.ErrorSummaries) > 0 {
		metadata["errors"] = summary.ErrorSummaries
	}
	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		body = []byte("{}")
	}
	message := fmt.Sprintf(
		"DOT_PROJECT_SYNC_RUN: scanned=%d synced=%d errored=%d github_errors=%d rate_limit_errors=%d skipped=%d",
		summary.Total,
		summary.Synced,
		summary.Errored,
		summary.GitHubErrorCount,
		summary.RateLimitErrorCount,
		summary.Skipped,
	)
	return model.AuditLog{
		Action:   "DOT_PROJECT_SYNC_RUN",
		Message:  message,
		Metadata: string(body),
	}
}
