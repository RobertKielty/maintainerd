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

type postSyncMetrics struct {
	DBSizeBytes              int64
	DotProjectSyncStateBytes int64
	CachedFiles              int64
	MaintainersBodyBytes     int64
	AvgMaintainersBodyBytes  int64
	MaxMaintainersBodyBytes  int64
	ProjectsTotal            int64
	ReposFound               int64
	CachedBodies             int64
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dbDriver := envOr("MD_DB_DRIVER", "postgres")
	if dbDriver != "postgres" {
		log.Fatalf("dot-project-sync requires MD_DB_DRIVER=postgres, got %q", dbDriver)
	}
	dbDSN := envOr("MD_DB_DSN", "")
	if dbDSN == "" {
		log.Fatal("MD_DB_DSN is required when MD_DB_DRIVER=postgres")
	}

	githubToken := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	if githubToken == "" {
		log.Fatal("GITHUB_API_TOKEN is required")
	}

	dbConn, err := db.OpenGorm(dbDriver, dbDSN, &gorm.Config{})
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

	metrics, metricsErr := collectPostSyncMetrics(ctx, store)
	if metricsErr != nil {
		log.Printf("dot-project sync post-sync metrics failed: %v", metricsErr)
	}

	logger := zap.NewNop().Sugar()
	if err := store.LogAuditEvent(logger, buildAuditEvent(summary, metrics, metricsErr)); err != nil {
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

func collectPostSyncMetrics(ctx context.Context, store *db.SQLStore) (*postSyncMetrics, error) {
	if store == nil || store.DB() == nil {
		return nil, fmt.Errorf("sql store is not initialized")
	}
	if store.DB().Name() != "postgres" {
		return nil, fmt.Errorf("dot-project-sync post-sync metrics require postgres, got %q", store.DB().Name())
	}

	metrics := &postSyncMetrics{}

	sizeRow := struct {
		DBSizeBytes              int64
		DotProjectSyncStateBytes int64
	}{}
	if err := store.DB().WithContext(ctx).Raw(`
		select
			pg_database_size(current_database()) as db_size_bytes,
			pg_total_relation_size('dot_project_sync_states') as dot_project_sync_state_bytes
	`).Scan(&sizeRow).Error; err != nil {
		return metrics, err
	}
	metrics.DBSizeBytes = sizeRow.DBSizeBytes
	metrics.DotProjectSyncStateBytes = sizeRow.DotProjectSyncStateBytes

	bodyRow := struct {
		CachedFiles             int64
		MaintainersBodyBytes    int64
		AvgMaintainersBodyBytes int64
		MaxMaintainersBodyBytes int64
	}{}
	if err := store.DB().WithContext(ctx).Raw(`
		select
			count(*) filter (where maintainers_file_body is not null) as cached_files,
			coalesce(sum(pg_column_size(maintainers_file_body)), 0) as maintainers_body_bytes,
			coalesce(avg(pg_column_size(maintainers_file_body))::bigint, 0) as avg_maintainers_body_bytes,
			coalesce(max(pg_column_size(maintainers_file_body)), 0) as max_maintainers_body_bytes
		from dot_project_sync_states
	`).Scan(&bodyRow).Error; err != nil {
		return metrics, err
	}
	metrics.CachedFiles = bodyRow.CachedFiles
	metrics.MaintainersBodyBytes = bodyRow.MaintainersBodyBytes
	metrics.AvgMaintainersBodyBytes = bodyRow.AvgMaintainersBodyBytes
	metrics.MaxMaintainersBodyBytes = bodyRow.MaxMaintainersBodyBytes

	coverageRow := struct {
		ProjectsTotal int64
		ReposFound    int64
		CachedBodies  int64
	}{}
	if err := store.DB().WithContext(ctx).Raw(`
		select
			count(*) as projects_total,
			count(*) filter (where repo_exists) as repos_found,
			count(*) filter (where maintainers_file_body is not null) as cached_bodies
		from dot_project_sync_states
	`).Scan(&coverageRow).Error; err != nil {
		return metrics, err
	}
	metrics.ProjectsTotal = coverageRow.ProjectsTotal
	metrics.ReposFound = coverageRow.ReposFound
	metrics.CachedBodies = coverageRow.CachedBodies

	return metrics, nil
}

func buildAuditEvent(summary dotproject.SyncSummary, metrics *postSyncMetrics, metricsErr error) model.AuditLog {
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
	if metrics != nil {
		metadata["db_size_bytes"] = metrics.DBSizeBytes
		metadata["dot_project_sync_state_bytes"] = metrics.DotProjectSyncStateBytes
		metadata["cached_files"] = metrics.CachedFiles
		metadata["maintainers_body_bytes"] = metrics.MaintainersBodyBytes
		metadata["avg_maintainers_body_bytes"] = metrics.AvgMaintainersBodyBytes
		metadata["max_maintainers_body_bytes"] = metrics.MaxMaintainersBodyBytes
		metadata["projects_total"] = metrics.ProjectsTotal
		metadata["repos_found"] = metrics.ReposFound
		metadata["cached_bodies"] = metrics.CachedBodies
	}
	if metricsErr != nil {
		metadata["db_metrics_error"] = strings.TrimSpace(metricsErr.Error())
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
