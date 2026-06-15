package lfx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"maintainerd/dotproject"
	"maintainerd/model"
)

type ObservationStore interface {
	GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error)
	ListMaintainersWithoutIdentityObservation(source string) ([]model.Maintainer, error)
	UpsertMaintainerIdentityObservation(observation *model.MaintainerIdentityObservation) (*model.MaintainerIdentityObservation, error)
}

type UserSearcher interface {
	SearchUsers(ctx context.Context, query UserSearch) ([]User, error)
	GetUserIdentities(ctx context.Context, salesforceID string) ([]Identity, error)
}

type Enricher struct {
	Store      ObservationStore
	Client     UserSearcher
	EnrichAll  bool
	MaxLookups int
	Now        func() time.Time
	Progress   func(EnrichmentProgress)

	allMaintainersDone bool
}

type EnrichmentProgress struct {
	Total     int
	Processed int
	Current   string
	Summary   dotproject.EnrichmentSummary
}

func (e *Enricher) EnrichProject(ctx context.Context, project model.Project, result *dotproject.DiscoveryResult) (dotproject.EnrichmentSummary, error) {
	if e == nil || e.Store == nil || e.Client == nil {
		return dotproject.EnrichmentSummary{}, fmt.Errorf("lfx enricher store and client are required")
	}
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}

	summary := dotproject.EnrichmentSummary{}
	candidates, err := e.candidates(project, result)
	if err != nil {
		return summary, err
	}
	total := len(candidates)
	e.reportProgress(total, 0, "", summary)
	processed := 0
	for _, candidate := range candidates {
		current := candidateLabel(candidate)
		if e.MaxLookups > 0 && summary.Attempted >= e.MaxLookups {
			summary.SkippedLimit++
			processed++
			e.reportProgress(total, processed, current, summary)
			continue
		}
		projectID := &project.ID
		if candidate.Global {
			projectID = nil
		}
		if err := e.enrichCandidate(ctx, projectID, candidate, now, &summary); err != nil {
			summary.Errored++
			processed++
			e.reportProgress(total, processed, current, summary)
			return summary, err
		}
		processed++
		e.reportProgress(total, processed, current, summary)
	}
	if e.EnrichAll && summary.SkippedLimit == 0 {
		e.allMaintainersDone = true
	}
	return summary, nil
}

func (e *Enricher) reportProgress(total, processed int, current string, summary dotproject.EnrichmentSummary) {
	if e == nil || e.Progress == nil {
		return
	}
	e.Progress(EnrichmentProgress{
		Total:     total,
		Processed: processed,
		Current:   current,
		Summary:   summary,
	})
}

type candidate struct {
	Maintainer *model.Maintainer
	GitHubUser string
	Email      string
	SourceRef  string
	Global     bool
}

func (e *Enricher) candidates(project model.Project, result *dotproject.DiscoveryResult) ([]candidate, error) {
	seen := make(map[string]struct{})
	candidates := make([]candidate, 0)
	if !e.EnrichAll && result != nil && result.MaintainersFile.Exists {
		byHandle, err := e.Store.GetMaintainerMapByGitHubAccount()
		if err != nil {
			return nil, err
		}
		handles, status, _ := dotproject.ParseProjectMaintainerHandles(result.MaintainersFile.Body)
		if status != dotproject.ParseStatusParsed {
			return candidates, nil
		}
		for _, handle := range handles {
			key := strings.ToLower(strings.TrimSpace(handle))
			if key == "" {
				continue
			}
			maintainer, ok := byHandle[key]
			if ok {
				candidates = appendCandidate(candidates, seen, candidate{
					Maintainer: &maintainer,
					GitHubUser: maintainer.GitHubAccount,
					Email:      preferredEmail(maintainer),
					SourceRef:  "github:" + key,
				})
				continue
			}
			candidates = appendCandidate(candidates, seen, candidate{
				GitHubUser: key,
				SourceRef:  "github:" + key,
			})
		}
	}

	if e.EnrichAll && !e.allMaintainersDone {
		maintainers, err := e.Store.ListMaintainersWithoutIdentityObservation("lfx")
		if err != nil {
			return nil, err
		}
		for _, maintainer := range maintainers {
			if maintainer.MaintainerStatus != "" && maintainer.MaintainerStatus != model.ActiveMaintainer {
				continue
			}
			candidates = appendCandidate(candidates, seen, candidate{
				Maintainer: &maintainer,
				GitHubUser: maintainer.GitHubAccount,
				Email:      preferredEmail(maintainer),
				SourceRef:  "maintainer-d:" + fmt.Sprintf("%d", maintainer.ID),
				Global:     true,
			})
		}
	}

	_ = project
	return candidates, nil
}

func appendCandidate(candidates []candidate, seen map[string]struct{}, candidate candidate) []candidate {
	key := candidate.SourceRef
	if candidate.Maintainer != nil {
		key = fmt.Sprintf("maintainer:%d", candidate.Maintainer.ID)
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return candidates
	}
	if _, ok := seen[key]; ok {
		return candidates
	}
	seen[key] = struct{}{}
	return append(candidates, candidate)
}

func candidateLabel(candidate candidate) string {
	if candidate.Maintainer != nil {
		if github := strings.TrimSpace(candidate.Maintainer.GitHubAccount); github != "" {
			return github
		}
		if email := strings.TrimSpace(candidate.Maintainer.Email); email != "" {
			return email
		}
		return fmt.Sprintf("maintainer:%d", candidate.Maintainer.ID)
	}
	if github := strings.TrimSpace(candidate.GitHubUser); github != "" {
		return github
	}
	if email := strings.TrimSpace(candidate.Email); email != "" {
		return email
	}
	return strings.TrimSpace(candidate.SourceRef)
}

func (e *Enricher) enrichCandidate(ctx context.Context, projectID *uint, candidate candidate, now time.Time, summary *dotproject.EnrichmentSummary) error {
	githubUser := normalized(candidate.GitHubUser)
	email := normalized(candidate.Email)
	if githubUser == "" || githubUser == "github_missing" {
		githubUser = ""
	}
	if email == "" || email == "email_missing" || email == "github_email_missing" {
		email = ""
	}
	if githubUser == "" && email == "" {
		summary.Unmatched++
		return e.writeObservation(projectID, candidate, nil, nil, now, "unmatched", "no github handle or email available", "")
	}

	summary.Attempted++
	users, err := e.searchUsers(ctx, githubUser, email)
	if err != nil {
		if writeErr := e.writeObservation(projectID, candidate, nil, nil, now, "error", err.Error(), ""); writeErr != nil {
			return fmt.Errorf("%w; failed to record LFX error observation: %v", PlatformAccessError(err), writeErr)
		}
		return PlatformAccessError(err)
	}
	switch len(users) {
	case 0:
		summary.Unmatched++
		return e.writeObservation(projectID, candidate, nil, nil, now, "unmatched", "no LFX user matched github handle or email", "")
	case 1:
		summary.Matched++
		user := users[0]
		identities, err := e.Client.GetUserIdentities(ctx, user.ID)
		if err != nil {
			if writeErr := e.writeObservation(projectID, candidate, nil, nil, now, "error", err.Error(), ""); writeErr != nil {
				return fmt.Errorf("%w; failed to record LFX error observation: %v", PlatformAccessError(err), writeErr)
			}
			return PlatformAccessError(err)
		}
		confidence := confidenceFor(user, identities, githubUser, email)
		return e.writeObservation(projectID, candidate, &user, identities, now, "matched", "single LFX user match", confidence)
	default:
		summary.Ambiguous++
		raw, err := json.Marshal(users)
		if err != nil {
			return err
		}
		return e.writeRawObservation(projectID, candidate, now, "ambiguous", "multiple LFX users matched github handle or email", "weak", string(raw))
	}
}

func (e *Enricher) searchUsers(ctx context.Context, githubUser, email string) ([]User, error) {
	if githubUser != "" {
		users, err := e.Client.SearchUsers(ctx, UserSearch{GitHubID: githubUser, PageSize: 10})
		if err != nil {
			return nil, err
		}
		if len(users) > 0 {
			return users, nil
		}
	}
	if email != "" {
		return e.Client.SearchUsers(ctx, UserSearch{Email: email, PageSize: 10})
	}
	return nil, nil
}

func (e *Enricher) writeObservation(projectID *uint, candidate candidate, user *User, identities []Identity, now time.Time, status, reason, confidence string) error {
	rawPayload := ""
	if user != nil {
		raw := struct {
			User       User       `json:"user"`
			Identities []Identity `json:"identities,omitempty"`
		}{User: *user, Identities: identities}
		body, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		rawPayload = string(body)
	}
	return e.writeRawObservation(projectID, candidate, now, status, reason, confidence, rawPayload)
}

func (e *Enricher) writeRawObservation(projectID *uint, candidate candidate, now time.Time, status, reason, confidence, rawPayload string) error {
	var maintainerID *uint
	if candidate.Maintainer != nil {
		id := candidate.Maintainer.ID
		maintainerID = &id
	}
	observation := &model.MaintainerIdentityObservation{
		MaintainerID: maintainerID,
		ProjectID:    projectID,
		Source:       "lfx",
		SourceRef:    candidate.SourceRef,
		GitHubUser:   strings.TrimSpace(candidate.GitHubUser),
		Email:        strings.TrimSpace(candidate.Email),
		MatchStatus:  status,
		MatchReason:  reason,
		Confidence:   confidence,
		RawPayload:   rawPayload,
		ObservedAt:   now,
	}
	if rawPayload != "" {
		var payload struct {
			User User `json:"user"`
		}
		if err := json.Unmarshal([]byte(rawPayload), &payload); err == nil {
			observation.SourceUserID = strings.TrimSpace(payload.User.ID)
			observation.Name = firstNonEmpty(payload.User.Name, strings.TrimSpace(payload.User.FirstName+" "+payload.User.LastName))
			observation.Email = firstNonEmpty(payload.User.Email, observation.Email)
			observation.LFID = payload.User.Username
			observation.CompanyName = accountCompanyName(payload.User.Account)
		}
	}
	_, err := e.Store.UpsertMaintainerIdentityObservation(observation)
	return err
}

func confidenceFor(user User, identities []Identity, githubUser, email string) string {
	for _, identity := range identities {
		if strings.EqualFold(strings.TrimSpace(identity.Source), "github") && githubUser != "" && strings.EqualFold(identity.Username, githubUser) {
			return "exact"
		}
	}
	if email != "" && strings.EqualFold(user.Email, email) {
		return "strong"
	}
	if githubUser != "" {
		return "strong"
	}
	return "weak"
}

func preferredEmail(maintainer model.Maintainer) string {
	if email := strings.TrimSpace(maintainer.Email); email != "" && email != "EMAIL_MISSING" {
		return email
	}
	if email := strings.TrimSpace(maintainer.GitHubEmail); email != "" && email != "GITHUB_EMAIL_MISSING" && email != "GITHUB_MISSING" {
		return email
	}
	return ""
}

func normalized(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func accountCompanyName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return firstNonEmpty(
		mapStringValue(payload, "Company"),
		mapStringValue(payload, "company"),
		mapStringValue(payload, "CompanyName"),
		mapStringValue(payload, "companyName"),
		mapStringValue(payload, "Name"),
		mapStringValue(payload, "name"),
	)
}

func mapStringValue(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return firstNonEmpty(
			mapStringValue(typed, "Name"),
			mapStringValue(typed, "name"),
			mapStringValue(typed, "CompanyName"),
			mapStringValue(typed, "companyName"),
		)
	default:
		return ""
	}
}
