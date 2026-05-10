package dotproject

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"maintainerd/model"

	"github.com/google/go-github/v55/github"
)

const (
	AdoptionStatusNotFound = "not_found"
	AdoptionStatusRepoOnly = "repo_only"
	AdoptionStatusPartial  = "partial"
	AdoptionStatusAdopted  = "adopted"
	AdoptionStatusError    = "error"
)

type SyncStore interface {
	ListProjects() ([]model.Project, error)
	PersistDotProjectSync(projectID uint, patch model.Project, state *model.DotProjectSyncState) error
}

type DiscoveryRunner interface {
	Discover(ctx context.Context, project model.Project) (*DiscoveryResult, error)
}

type SyncSummary struct {
	Loaded              int
	Total               int
	Skipped             int
	SkippedArchived     int
	SkippedExcluded     int
	Synced              int
	Errored             int
	NotFound            int
	Adopted             int
	Partial             int
	RepoOnly            int
	GitHubErrorCount    int
	RateLimitErrorCount int
	ErrorSummaries      []string
}

type Syncer struct {
	Store      SyncStore
	Discoverer DiscoveryRunner
	Now        func() time.Time
}

func (s *Syncer) SyncAll(ctx context.Context) (SyncSummary, error) {
	if s == nil || s.Store == nil || s.Discoverer == nil {
		return SyncSummary{}, fmt.Errorf("dot-project syncer store and discoverer are required")
	}

	projects, err := s.Store.ListProjects()
	if err != nil {
		return SyncSummary{}, err
	}

	summary := SyncSummary{Loaded: len(projects)}
	for _, project := range projects {
		if !shouldSyncProject(project) {
			summary.Skipped++
			if isArchivedProject(project) {
				summary.SkippedArchived++
			} else {
				summary.SkippedExcluded++
			}
			continue
		}
		summary.Total++
		status, err := s.SyncProject(ctx, project)
		if err != nil {
			summary.Errored++
			summary.recordError(err)
			continue
		}
		summary.Synced++
		switch status {
		case AdoptionStatusNotFound:
			summary.NotFound++
		case AdoptionStatusAdopted:
			summary.Adopted++
		case AdoptionStatusPartial:
			summary.Partial++
		case AdoptionStatusRepoOnly:
			summary.RepoOnly++
		}
	}
	return summary, nil
}

func (s *SyncSummary) recordError(err error) {
	if err == nil {
		return
	}
	if isGitHubRateLimitError(err) {
		s.RateLimitErrorCount++
		s.GitHubErrorCount++
	} else if isGitHubAPIError(err) {
		s.GitHubErrorCount++
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return
	}
	for _, existing := range s.ErrorSummaries {
		if existing == message {
			return
		}
	}
	if len(s.ErrorSummaries) >= 10 {
		return
	}
	s.ErrorSummaries = append(s.ErrorSummaries, message)
}

func (s *Syncer) SyncProject(ctx context.Context, project model.Project) (string, error) {
	now := time.Now().UTC()
	if s != nil && s.Now != nil {
		now = s.Now().UTC()
	}

	discoveryProject := project
	if inferredOrg, ok := inferGitHubOrg(project); ok {
		discoveryProject.GitHubOrg = inferredOrg
	}

	result, err := s.Discoverer.Discover(ctx, discoveryProject)
	if err != nil {
		status := AdoptionStatusError
		state := &model.DotProjectSyncState{
			ProjectID:       project.ID,
			ImporterVersion: ImporterVersion,
			LastCheckedAt:   &now,
			SyncError:       stringPtr(err.Error()),
		}
		patch := project
		patch.DotProjectLastSyncedAt = &now
		patch.DotProjectAdoptionStatus = status
		persistErr := s.Store.PersistDotProjectSync(project.ID, patch, state)
		if persistErr != nil {
			return status, fmt.Errorf("persist sync error for project %d: %w", project.ID, persistErr)
		}
		return status, err
	}

	status := adoptionStatusFor(result)
	state := buildSyncState(project.ID, now, result)
	patch := buildProjectPatch(now, status, result)
	if err := s.Store.PersistDotProjectSync(project.ID, patch, state); err != nil {
		return status, err
	}
	return status, nil
}

func buildProjectPatch(now time.Time, status string, result *DiscoveryResult) model.Project {
	patch := model.Project{
		DotProjectLastSyncedAt:   &now,
		DotProjectAdoptionStatus: status,
	}
	if result == nil {
		return patch
	}
	patch.DotProjectRepoRef = strings.TrimSpace(result.RepoRef)
	patch.DotProjectProjectRef = strings.TrimSpace(result.ProjectFile.BlobURL)
	patch.DotProjectMaintainerRef = strings.TrimSpace(result.MaintainersFile.BlobURL)
	patch.DotProjectSecurityRef = strings.TrimSpace(result.SecurityFile.BlobURL)
	patch.DotProjectContributingRef = strings.TrimSpace(result.ContributingFile.BlobURL)
	patch.DotProjectGovernanceRef = strings.TrimSpace(result.GovernanceFile.BlobURL)
	patch.DotProjectSchemaVersion = strings.TrimSpace(result.SchemaVersion)
	patch.DotProjectMaintainerCount = result.MaintainerCount
	if !result.RepoExists {
		patch.DotProjectRepoRef = ""
	}
	return patch
}

func buildSyncState(projectID uint, now time.Time, result *DiscoveryResult) *model.DotProjectSyncState {
	state := &model.DotProjectSyncState{
		ProjectID:       projectID,
		ImporterVersion: ImporterVersion,
		LastCheckedAt:   &now,
	}
	if result == nil {
		return state
	}

	state.RepoExists = result.RepoExists
	state.ProjectFileExists = result.ProjectFile.Exists
	state.MaintainersFileExists = result.MaintainersFile.Exists
	state.SecurityFileExists = result.SecurityFile.Exists
	state.ContributingFileExists = result.ContributingFile.Exists
	state.GovernanceFileExists = result.GovernanceFile.Exists
	state.DefaultBranch = strings.TrimSpace(result.DefaultBranch)
	state.MaintainersFilename = strings.TrimSpace(result.MaintainersFilename)
	state.SchemaVersion = strings.TrimSpace(result.SchemaVersion)
	state.ProjectFileETag = strings.TrimSpace(result.ProjectFile.ETag)
	state.MaintainersFileETag = strings.TrimSpace(result.MaintainersFile.ETag)
	state.SecurityFileETag = strings.TrimSpace(result.SecurityFile.ETag)
	state.ContributingFileETag = strings.TrimSpace(result.ContributingFile.ETag)
	state.GovernanceFileETag = strings.TrimSpace(result.GovernanceFile.ETag)
	state.ProjectFileBodyHash = strings.TrimSpace(result.ProjectFile.BodyHash)
	state.MaintainersFileBodyHash = strings.TrimSpace(result.MaintainersFile.BodyHash)
	state.SecurityFileBodyHash = strings.TrimSpace(result.SecurityFile.BodyHash)
	state.ContributingFileBodyHash = strings.TrimSpace(result.ContributingFile.BodyHash)
	state.GovernanceFileBodyHash = strings.TrimSpace(result.GovernanceFile.BodyHash)
	if parseErr := joinParseErrors(result); parseErr != "" {
		state.ParseError = &parseErr
	}
	return state
}

func adoptionStatusFor(result *DiscoveryResult) string {
	if result == nil || !result.RepoExists {
		return AdoptionStatusNotFound
	}
	hasAnyFile := result.ProjectFile.Exists || result.MaintainersFile.Exists || result.SecurityFile.Exists || result.ContributingFile.Exists || result.GovernanceFile.Exists
	if !hasAnyFile {
		return AdoptionStatusRepoOnly
	}
	if result.ProjectFile.Exists && result.MaintainersFile.Exists && result.SchemaSupported && result.MaintainerCount != nil {
		return AdoptionStatusAdopted
	}
	return AdoptionStatusPartial
}

func joinParseErrors(result *DiscoveryResult) string {
	if result == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if msg := strings.TrimSpace(result.ProjectParseError); msg != "" {
		parts = append(parts, "project.yaml: "+msg)
	}
	if msg := strings.TrimSpace(result.MaintainersParseError); msg != "" {
		parts = append(parts, result.MaintainersFilename+": "+msg)
	}
	return strings.Join(parts, "; ")
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func inferGitHubOrg(project model.Project) (string, bool) {
	if org := strings.TrimSpace(project.GitHubOrg); org != "" {
		return org, true
	}
	ref := strings.TrimSpace(project.LegacyMaintainerRef)
	if ref == "" {
		return "", false
	}
	return parseGitHubOrgFromURL(ref)
}

func parseGitHubOrgFromURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Host)
	if host != "github.com" && host != "www.github.com" && host != "raw.githubusercontent.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", false
	}
	org := strings.TrimSpace(parts[0])
	if org == "" {
		return "", false
	}
	return org, true
}

func shouldSyncProject(project model.Project) bool {
	if isArchivedProject(project) {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(project.Name))
	return name != "maintainer-d"
}

func isArchivedProject(project model.Project) bool {
	return project.Maturity == model.Archived
}

func isGitHubRateLimitError(err error) bool {
	var rateLimitErr *github.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return true
	}
	var abuseRateLimitErr *github.AbuseRateLimitError
	return errors.As(err, &abuseRateLimitErr)
}

func isGitHubAPIError(err error) bool {
	if isGitHubRateLimitError(err) {
		return true
	}
	var responseErr *github.ErrorResponse
	if errors.As(err, &responseErr) {
		return true
	}
	var acceptedErr *github.AcceptedError
	return errors.As(err, &acceptedErr)
}
