package dotproject

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"maintainerd/model"

	"go.uber.org/zap"
)

type AutoAddStore interface {
	GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error)
	GetMaintainersByProject(projectID uint) ([]model.Maintainer, error)
	UpsertMaintainerWithIdentity(projectID uint, name, email, githubHandle, company, lfxUserID string) (*model.Maintainer, bool, bool, error)
	UpsertMaintainerIdentityObservation(observation *model.MaintainerIdentityObservation) (*model.MaintainerIdentityObservation, error)
	GetLatestMaintainerIdentityObservation(source string, maintainerID uint) (*model.MaintainerIdentityObservation, error)
	GetLatestMaintainerIdentityObservationByRef(source string, projectID uint, sourceRef string) (*model.MaintainerIdentityObservation, error)
	LogAuditEvent(logger *zap.SugaredLogger, event model.AuditLog) error
}

type LFXIdentityResolver interface {
	ResolveMaintainerIdentity(ctx context.Context, githubHandle, email string) (LFXIdentityResult, error)
}

type LFXIdentityResult struct {
	UserID     string
	LFID       string
	Name       string
	Email      string
	Company    string
	GitHubUser string
	Confidence string
	Reason     string
	RawPayload string
}

type AutoMaintainerAdder struct {
	Store              AutoAddStore
	Foundation         *FoundationMaintainerIndex
	LFX                LFXIdentityResolver
	Actor              string
	CheckFoundationCSV bool
	AutoAddMaintainers bool
	Now                func() time.Time
	Logger             *zap.SugaredLogger
}

type AutoAddSummary struct {
	Candidates                int
	DryRunCandidates          int
	CreatedMaintainers        int
	LinkedMaintainers         int
	WouldCreateMaintainers    int
	WouldLinkMaintainers      int
	SkippedFoundationMissing  int
	SkippedCSVLoadFailed      int
	SkippedProjectMismatch    int
	SkippedInvalidMaintainers int
	LFXAttempted              int
	LFXMatched                int
	LFXUnmatched              int
	LFXErrored                int
	Errored                   int
	AuditFailures             int
	WouldCreate               []AutoAddCandidateSummary
	WouldLink                 []AutoAddCandidateSummary
}

type AutoAddCandidateSummary struct {
	ProjectID uint   `json:"project_id"`
	Project   string `json:"project"`
	GitHub    string `json:"github"`
	LFXID     string `json:"lfx_id"`
	Name      string `json:"name"`
	Company   string `json:"company"`
	Email     string `json:"email"`
}

func (s *AutoAddSummary) add(other AutoAddSummary) {
	s.Candidates += other.Candidates
	s.DryRunCandidates += other.DryRunCandidates
	s.CreatedMaintainers += other.CreatedMaintainers
	s.LinkedMaintainers += other.LinkedMaintainers
	s.WouldCreateMaintainers += other.WouldCreateMaintainers
	s.WouldLinkMaintainers += other.WouldLinkMaintainers
	s.SkippedFoundationMissing += other.SkippedFoundationMissing
	s.SkippedCSVLoadFailed += other.SkippedCSVLoadFailed
	s.SkippedProjectMismatch += other.SkippedProjectMismatch
	s.SkippedInvalidMaintainers += other.SkippedInvalidMaintainers
	s.LFXAttempted += other.LFXAttempted
	s.LFXMatched += other.LFXMatched
	s.LFXUnmatched += other.LFXUnmatched
	s.LFXErrored += other.LFXErrored
	s.Errored += other.Errored
	s.AuditFailures += other.AuditFailures
	s.WouldCreate = append(s.WouldCreate, other.WouldCreate...)
	s.WouldLink = append(s.WouldLink, other.WouldLink...)
}

func (a *AutoMaintainerAdder) ProcessProject(ctx context.Context, project model.Project, result *DiscoveryResult) (AutoAddSummary, error) {
	summary := AutoAddSummary{}
	if a == nil || a.Store == nil {
		return summary, fmt.Errorf("auto maintainer adder store is required")
	}
	if result == nil || !result.MaintainersFile.Exists {
		return summary, nil
	}
	if a.CheckFoundationCSV && a.Foundation == nil {
		summary.SkippedCSVLoadFailed++
		return summary, fmt.Errorf("foundation csv gate is enabled but foundation csv was not loaded")
	}

	handles, status, parseErr := ParseProjectMaintainerHandles(result.MaintainersFile.Body)
	if status != ParseStatusParsed {
		summary.SkippedInvalidMaintainers++
		if a.Logger != nil {
			a.Logger.Infow(
				"skipping auto-add for invalid project-maintainers file",
				"project_id", project.ID,
				"project_name", project.Name,
				"maintainers_url", fileDiscoveryURL(result.MaintainersFile),
				"parse_status", status,
				"parse_error", parseErr,
			)
		}
		return summary, nil
	}

	byHandle, err := a.Store.GetMaintainerMapByGitHubAccount()
	if err != nil {
		return summary, err
	}
	projectMaintainers, err := a.Store.GetMaintainersByProject(project.ID)
	if err != nil {
		return summary, err
	}
	linked := make(map[uint]struct{}, len(projectMaintainers))
	for _, maintainer := range projectMaintainers {
		linked[maintainer.ID] = struct{}{}
	}

	now := time.Now().UTC()
	if a.Now != nil {
		now = a.Now().UTC()
	}

	for _, handle := range handles {
		normalized := NormalizeGitHubHandle(handle)
		if normalized == "" || isPlaceholderGitHubHandle(normalized) {
			continue
		}
		summary.Candidates++
		if !a.AutoAddMaintainers {
			summary.DryRunCandidates++
		}

		existing, hasExisting := byHandle[normalized]
		var maintainerID *uint
		if hasExisting {
			id := existing.ID
			maintainerID = &id
		}

		record := FoundationMaintainerRecord{
			Project: strings.TrimSpace(project.Name),
			GitHub:  handle,
		}
		if a.CheckFoundationCSV {
			var ok bool
			record, ok = a.Foundation.Lookup(project.Name, normalized)
			if !ok {
				record = FoundationMaintainerRecord{
					Project: strings.TrimSpace(project.Name),
					GitHub:  normalized,
				}
				reason := normalized + " not found in cncf/foundation project-maintainers.csv for project " + project.Name
				if a.Foundation.HasGitHub(normalized) {
					summary.SkippedProjectMismatch++
					reason = normalized + " exists in cncf/foundation project-maintainers.csv under a different project"
				} else {
					summary.SkippedFoundationMissing++
				}
				if err := a.writeFoundationObservation(project.ID, maintainerID, normalized, record, now, "unmatched", reason, "", nil); err != nil {
					summary.AuditFailures++
				}
				continue
			}
		}

		if err := a.writeFoundationObservation(project.ID, maintainerID, normalized, record, now, "matched", "present in cncf/foundation project-maintainers.csv", "strong", nil); err != nil {
			summary.AuditFailures++
		}

		if hasExisting {
			if _, ok := linked[existing.ID]; ok {
				continue
			}
			entry := a.candidateSummary(project, record, normalized, &existing, nil)
			if !a.AutoAddMaintainers {
				summary.WouldLinkMaintainers++
				summary.WouldLink = append(summary.WouldLink, entry)
				if err := a.logAutoAdd(project, &existing, record, result, nil, now, "would_link"); err != nil {
					summary.AuditFailures++
				}
				continue
			}
			maintainer, _, didLink, err := a.Store.UpsertMaintainerWithIdentity(project.ID, existing.Name, existing.Email, existing.GitHubAccount, companyName(existing.Company.Name, record.Company), existing.LFXUserID)
			if err != nil {
				return summary, err
			}
			if didLink {
				summary.LinkedMaintainers++
				if err := a.logAutoAdd(project, maintainer, record, result, nil, now, "linked"); err != nil {
					summary.AuditFailures++
				}
			}
			continue
		}

		identity := LFXIdentityResult{}
		if a.LFX != nil {
			summary.LFXAttempted++
			resolved, err := a.LFX.ResolveMaintainerIdentity(ctx, normalized, "")
			if err != nil {
				summary.LFXErrored++
				return summary, err
			} else {
				identity = resolved
				if isUsableLFXConfidence(identity.Confidence) {
					summary.LFXMatched++
				} else {
					summary.LFXUnmatched++
				}
			}
		}

		entry := a.candidateSummary(project, record, normalized, nil, &identity)
		if !a.AutoAddMaintainers {
			summary.WouldCreateMaintainers++
			summary.WouldCreate = append(summary.WouldCreate, entry)
			if err := a.logAutoAdd(project, nil, record, result, &identity, now, "would_create"); err != nil {
				summary.AuditFailures++
			}
			continue
		}

		email := "EMAIL_MISSING"
		lfxUserID := ""
		if isUsableLFXConfidence(identity.Confidence) {
			if value := strings.TrimSpace(identity.Email); value != "" {
				email = value
			}
			lfxUserID = strings.TrimSpace(identity.UserID)
		}
		maintainer, created, didLink, err := a.Store.UpsertMaintainerWithIdentity(project.ID, record.Name, email, displayGitHub(record, normalized), record.Company, lfxUserID)
		if err != nil {
			return summary, err
		}
		if created {
			summary.CreatedMaintainers++
		} else if didLink {
			summary.LinkedMaintainers++
		}
		if created || didLink {
			if err := a.logAutoAdd(project, maintainer, record, result, &identity, now, actionFor(created)); err != nil {
				summary.AuditFailures++
			}
		}
	}
	return summary, nil
}

func (a *AutoMaintainerAdder) writeFoundationObservation(projectID uint, maintainerID *uint, github string, record FoundationMaintainerRecord, now time.Time, status, reason, confidence string, extra map[string]any) error {
	payload := map[string]any{
		"row":        record.Raw,
		"project":    record.Project,
		"name":       record.Name,
		"company":    record.Company,
		"github":     record.GitHub,
		"line":       record.LineNumber,
		"source_url": "",
		"commit_sha": "",
		"line_url":   "",
	}
	if a.Foundation != nil {
		payload["source_url"] = a.Foundation.SourceURL
		payload["commit_sha"] = a.Foundation.CommitSHA
		payload["line_url"] = a.Foundation.LineURL(record)
	}
	for key, value := range extra {
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	pid := projectID
	observation := &model.MaintainerIdentityObservation{
		MaintainerID: maintainerID,
		ProjectID:    &pid,
		Source:       FoundationCSVSource,
		SourceRef:    "github:" + NormalizeGitHubHandle(github),
		Name:         strings.TrimSpace(record.Name),
		GitHubUser:   displayGitHub(record, github),
		CompanyName:  strings.TrimSpace(record.Company),
		MatchStatus:  status,
		MatchReason:  reason,
		Confidence:   confidence,
		RawPayload:   string(raw),
		ObservedAt:   now,
	}
	_, err = a.Store.UpsertMaintainerIdentityObservation(observation)
	return err
}

func (a *AutoMaintainerAdder) logAutoAdd(project model.Project, maintainer *model.Maintainer, record FoundationMaintainerRecord, result *DiscoveryResult, identity *LFXIdentityResult, now time.Time, mode string) error {
	projectID := project.ID
	var maintainerID *uint
	var maintainerPath string
	maintainerName := strings.TrimSpace(record.Name)
	maintainerEmail := "EMAIL_MISSING"
	maintainerGitHub := displayGitHub(record, "")
	lfxID := ""
	if identity != nil {
		lfxID = firstCandidateValue(identity.LFID, identity.UserID)
		if value := strings.TrimSpace(identity.Name); value != "" {
			maintainerName = value
		}
		if value := strings.TrimSpace(identity.Email); value != "" {
			maintainerEmail = value
		}
	}
	if maintainer != nil {
		id := maintainer.ID
		maintainerID = &id
		maintainerPath = fmt.Sprintf("/maintainers/%d", maintainer.ID)
		if value := strings.TrimSpace(maintainer.Name); value != "" {
			maintainerName = value
		}
		if value := strings.TrimSpace(maintainer.Email); value != "" {
			maintainerEmail = value
		}
		if value := strings.TrimSpace(maintainer.GitHubAccount); value != "" {
			maintainerGitHub = value
		}
		if value := strings.TrimSpace(maintainer.LFXUserID); value != "" {
			lfxID = value
		}
	}
	if maintainerGitHub == "" {
		maintainerGitHub = displayGitHub(record, "")
	}
	modeLabel := autoAddModeLabel(mode)
	referenceRows := []map[string]any{
		{
			"label": "GitHub profile",
			"type":  "github_profile",
			"url":   githubProfileURL(maintainerGitHub),
		},
		{
			"label": "dot-project maintainers.yaml",
			"type":  "dot_project_maintainers_yaml",
			"url":   fileDiscoveryURL(resultMaintainersFile(result)),
		},
		{
			"label": "foundation project-maintainers.csv",
			"type":  "foundation_csv_line",
			"url":   a.foundationLineURL(record),
			"line":  record.LineNumber,
		},
	}
	if maintainerPath != "" {
		referenceRows = append(referenceRows, map[string]any{
			"label": "maintainer-d profile",
			"type":  "maintainer_profile",
			"url":   maintainerPath,
		})
	}
	metadata := map[string]any{
		"actor":                        a.auditActor(),
		"mode":                         mode,
		"mode_label":                   modeLabel,
		"project_id":                   project.ID,
		"project_name":                 project.Name,
		"github":                       maintainerGitHub,
		"name":                         maintainerName,
		"email":                        maintainerEmail,
		"lfx_id":                       lfxID,
		"source":                       ".project/maintainers.yaml project-maintainers",
		"source_url":                   fileDiscoveryURL(resultMaintainersFile(result)),
		"foundation_csv_gate_enabled":  a.CheckFoundationCSV,
		"auto_add_maintainers_enabled": a.AutoAddMaintainers,
		"foundation_csv_project":       record.Project,
		"foundation_csv_name":          record.Name,
		"foundation_csv_company":       record.Company,
		"foundation_csv_line":          record.LineNumber,
		"foundation_csv_line_url":      a.foundationLineURL(record),
		"references":                   referenceRows,
	}
	if maintainerID != nil {
		metadata["maintainer_id"] = *maintainerID
	}
	if a.Foundation != nil {
		metadata["foundation_csv_url"] = a.Foundation.SourceURL
		metadata["foundation_csv_commit_sha"] = a.Foundation.CommitSHA
	}
	if identity != nil && strings.TrimSpace(identity.Confidence) != "" {
		metadata["lfx_lookup_confidence"] = identity.Confidence
		metadata["lfx_lookup_reason"] = identity.Reason
	}
	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	event := model.AuditLog{
		ProjectID:    &projectID,
		MaintainerID: maintainerID,
		Action:       "ADD_DOT_PROJECT_MAINTAINER",
		Message:      a.autoAddMessage(maintainerGitHub, maintainerPath, lfxID, record, result, mode),
		Metadata:     string(body),
	}
	_ = now
	logger := a.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return a.Store.LogAuditEvent(logger, event)
}

func (a *AutoMaintainerAdder) autoAddMessage(githubHandle, maintainerPath, lfxID string, record FoundationMaintainerRecord, result *DiscoveryResult, mode string) string {
	verb := "was"
	if strings.HasPrefix(mode, "would_") {
		verb = "would have been"
	}
	kind := "new maintainer"
	if mode == "linked" || mode == "would_link" {
		kind = "existing maintainer linked"
	}
	emailSource := "Email address was not available from LFX"
	if strings.TrimSpace(lfxID) != "" {
		emailSource = fmt.Sprintf("Email address was found in LFX profile ID %s", lfxID)
	}
	maintainerRef := "maintainer-d profile will be created on write"
	if maintainerPath != "" {
		maintainerRef = maintainerPath
	}
	return fmt.Sprintf(
		"%s %s added to maintainer-d as a %s found in %s corroborated by %s. %s. %s",
		strings.TrimSpace(githubHandle),
		verb,
		kind,
		fileDiscoveryURL(resultMaintainersFile(result)),
		a.foundationLineURL(record),
		emailSource,
		maintainerRef,
	)
}

func (a *AutoMaintainerAdder) auditActor() string {
	if actor := strings.TrimSpace(a.Actor); actor != "" {
		return "dot-project-sync started by " + actor
	}
	return "dot-project-sync"
}

func (a *AutoMaintainerAdder) foundationLineURL(record FoundationMaintainerRecord) string {
	if a == nil || a.Foundation == nil {
		return ""
	}
	return a.Foundation.LineURL(record)
}

func resultMaintainersFile(result *DiscoveryResult) FileDiscovery {
	if result == nil {
		return FileDiscovery{}
	}
	return result.MaintainersFile
}

func fileDiscoveryURL(file FileDiscovery) string {
	if value := strings.TrimSpace(file.BlobURL); value != "" {
		return value
	}
	return strings.TrimSpace(file.RawURL)
}

func githubProfileURL(handle string) string {
	handle = strings.Trim(strings.TrimSpace(handle), "@")
	if handle == "" {
		return ""
	}
	return "https://github.com/" + handle
}

func autoAddModeLabel(mode string) string {
	switch mode {
	case "created":
		return "created new maintainer"
	case "linked":
		return "linked existing maintainer"
	case "would_create":
		return "would create new maintainer"
	case "would_link":
		return "would link existing maintainer"
	default:
		return mode
	}
}

func (a *AutoMaintainerAdder) candidateSummary(project model.Project, record FoundationMaintainerRecord, github string, existing *model.Maintainer, identity *LFXIdentityResult) AutoAddCandidateSummary {
	summary := AutoAddCandidateSummary{
		ProjectID: project.ID,
		Project:   project.Name,
		GitHub:    displayGitHub(record, github),
		Company:   strings.TrimSpace(record.Company),
	}
	if existing != nil {
		summary.LFXID = strings.TrimSpace(existing.LFXUserID)
		summary.Name = strings.TrimSpace(existing.Name)
		summary.Email = strings.TrimSpace(existing.Email)
		if company := strings.TrimSpace(existing.Company.Name); company != "" {
			summary.Company = company
		}
		if observation, err := a.Store.GetLatestMaintainerIdentityObservation("lfx", existing.ID); err == nil && observation != nil {
			applyObservationToCandidate(&summary, observation)
		}
		return summary
	}
	if identity != nil {
		summary.LFXID = firstCandidateValue(identity.LFID, identity.UserID)
		if value := strings.TrimSpace(identity.Name); value != "" {
			summary.Name = value
		} else {
			summary.Name = strings.TrimSpace(record.Name)
		}
		if value := strings.TrimSpace(identity.Company); value != "" {
			summary.Company = value
		}
		if value := strings.TrimSpace(identity.Email); value != "" {
			summary.Email = value
		}
	}
	sourceRef := "github:" + NormalizeGitHubHandle(github)
	if observation, err := a.Store.GetLatestMaintainerIdentityObservationByRef("lfx", project.ID, sourceRef); err == nil && observation != nil {
		applyObservationToCandidate(&summary, observation)
	}
	if summary.Name == "" {
		summary.Name = strings.TrimSpace(record.Name)
	}
	if summary.Company == "" {
		summary.Company = strings.TrimSpace(record.Company)
	}
	if summary.Email == "" {
		summary.Email = "EMAIL_MISSING"
	}
	return summary
}

func applyObservationToCandidate(summary *AutoAddCandidateSummary, observation *model.MaintainerIdentityObservation) {
	if summary == nil || observation == nil {
		return
	}
	if value := firstCandidateValue(observation.LFID, observation.SourceUserID); value != "" {
		summary.LFXID = value
	}
	if value := strings.TrimSpace(observation.Name); value != "" {
		summary.Name = value
	}
	if value := strings.TrimSpace(observation.CompanyName); value != "" {
		summary.Company = value
	}
	if value := strings.TrimSpace(observation.Email); value != "" {
		summary.Email = value
	}
}

func isPlaceholderGitHubHandle(handle string) bool {
	switch NormalizeGitHubHandle(handle) {
	case "github-handle", "github_missing":
		return true
	default:
		return false
	}
}

func isUsableLFXConfidence(confidence string) bool {
	return confidence == "exact" || confidence == "strong"
}

func displayGitHub(record FoundationMaintainerRecord, fallback string) string {
	if github := strings.TrimSpace(record.GitHub); github != "" {
		return github
	}
	return strings.TrimSpace(fallback)
}

func companyName(existing, observed string) string {
	if strings.TrimSpace(existing) != "" {
		return existing
	}
	return strings.TrimSpace(observed)
}

func firstCandidateValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func actionFor(created bool) string {
	if created {
		return "created"
	}
	return "linked"
}
