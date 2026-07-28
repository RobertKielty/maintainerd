// desentinel-links identifies (and, with -apply, severs) maintainer_projects
// links created by the sentinel-email collision bug in
// db.UpsertMaintainerWithIdentity: when a maintainer's identity could not be
// resolved, the auto-add flow looked up existing maintainers by
// email="EMAIL_MISSING" and matched whichever maintainer happened to hold
// that sentinel first, silently linking unrelated people's projects onto it
// instead of creating a distinct maintainer record.
//
// Detection happens in two tiers:
//
//  1. Proven: an ADD_DOT_PROJECT_MAINTAINER audit log entry with
//     mode="linked", email="EMAIL_MISSING", and a foundation_csv_name that
//     names a real, different person than the maintainer that got linked.
//     This is unambiguous - the audit trail itself names who the link
//     should have belonged to.
//  2. Corroborated: the same mode="linked"/email="EMAIL_MISSING" audit
//     signature, but the historical foundation-CSV cross-check was off so
//     no corroborating name was recorded. For these, the tool fetches the
//     project's current maintainers file (the same source cmd/sanitize
//     uses) and flags the link only if neither the maintainer's GitHub
//     handle nor name appears in it.
//
// In -apply mode the tool deletes the maintainer_projects row for every
// flagged link and records a SEVER_SENTINEL_MAINTAINER_LINK audit event
// referencing the evidence used.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"gorm.io/gorm"
)

type auditMetadata struct {
	Mode              string `json:"mode"`
	Email             string `json:"email"`
	FoundationCSVName string `json:"foundation_csv_name"`
	FoundationCompany string `json:"foundation_csv_company"`
	ProjectName       string `json:"project_name"`
}

type evidenceTier string

const (
	tierProven       evidenceTier = "proven"       // audit trail names the real, different person
	tierCorroborated evidenceTier = "corroborated" // live maintainers file doesn't mention this maintainer
)

type badLink struct {
	AuditLogID   uint
	MaintainerID uint
	ProjectID    uint
	ProjectName  string
	Tier         evidenceTier
	FoundName    string
	FoundCompany string
	Maintainer   model.Maintainer
}

func main() {
	apply := flag.Bool("apply", false, "sever the identified bad links (default: dry-run report only)")
	liveCheck := flag.Bool("live-check", true, "fetch each project's current maintainers file to corroborate links lacking a recorded foundation-CSV name")
	flag.Parse()

	dbDSN := envOr("MD_DB_DSN", "")
	if dbDSN == "" {
		log.Fatal("MD_DB_DSN is required")
	}

	conn, err := db.OpenGorm("postgres", dbDSN, &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}

	ctx := context.Background()
	links, err := findBadLinks(ctx, conn, *liveCheck)
	if err != nil {
		log.Fatalf("scan failed: %v", err)
	}

	if len(links) == 0 {
		fmt.Println("no sentinel-collision links found")
		return
	}

	fmt.Printf("found %d suspect maintainer-project link(s):\n\n", len(links))
	for _, l := range links {
		switch l.Tier {
		case tierProven:
			fmt.Printf("[proven]       audit_log_id=%d maintainer_id=%d (%s <%s>) project_id=%d (%s): audit trail says this link belongs to %q (%s)\n",
				l.AuditLogID, l.MaintainerID, l.Maintainer.Name, l.Maintainer.Email, l.ProjectID, l.ProjectName, l.FoundName, l.FoundCompany)
		case tierCorroborated:
			fmt.Printf("[corroborated] audit_log_id=%d maintainer_id=%d (%s <%s>) project_id=%d (%s): went through the sentinel-collision code path and does not appear in the project's current maintainers file\n",
				l.AuditLogID, l.MaintainerID, l.Maintainer.Name, l.Maintainer.Email, l.ProjectID, l.ProjectName)
		}
	}

	if !*apply {
		fmt.Println("\ndry-run only; re-run with -apply to sever these links")
		return
	}

	severed := 0
	for _, l := range links {
		ok, err := severLink(conn, l)
		if err != nil {
			log.Printf("failed to sever maintainer_id=%d project_id=%d: %v", l.MaintainerID, l.ProjectID, err)
			continue
		}
		if ok {
			severed++
		}
	}
	fmt.Printf("\nsevered %d link(s)\n", severed)
}

// findBadLinks scans ADD_DOT_PROJECT_MAINTAINER audit log entries for
// mode="linked" events against the EMAIL_MISSING sentinel - the signature of
// the sentinel-collision bug - and classifies each still-existing
// maintainer_projects row as proven or corroborated bad, or leaves it
// unflagged if neither check finds evidence.
func findBadLinks(ctx context.Context, conn *gorm.DB, liveCheck bool) ([]badLink, error) {
	var logs []model.AuditLog
	if err := conn.Where("action = ? AND maintainer_id IS NOT NULL AND project_id IS NOT NULL", "ADD_DOT_PROJECT_MAINTAINER").
		Order("id asc").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}

	maintainerCache := map[uint]*model.Maintainer{}
	projectCache := map[uint]*model.Project{}
	seen := map[[2]uint]bool{}
	var unproven []badLink
	var out []badLink

	loadMaintainer := func(id uint) (*model.Maintainer, error) {
		if m, ok := maintainerCache[id]; ok {
			return m, nil
		}
		var m model.Maintainer
		if err := conn.First(&m, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			return nil, err
		}
		maintainerCache[id] = &m
		return &m, nil
	}
	loadProject := func(id uint) (*model.Project, error) {
		if p, ok := projectCache[id]; ok {
			return p, nil
		}
		var p model.Project
		if err := conn.First(&p, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			return nil, err
		}
		projectCache[id] = &p
		return &p, nil
	}

	for _, entry := range logs {
		var meta auditMetadata
		if err := json.Unmarshal([]byte(entry.Metadata), &meta); err != nil {
			continue
		}
		if meta.Mode != "linked" || meta.Email != "EMAIL_MISSING" {
			continue
		}

		maintainerID := *entry.MaintainerID
		projectID := *entry.ProjectID
		key := [2]uint{maintainerID, projectID}
		if seen[key] {
			continue
		}

		maintainer, err := loadMaintainer(maintainerID)
		if err != nil {
			return nil, fmt.Errorf("load maintainer %d: %w", maintainerID, err)
		}
		if maintainer == nil {
			continue
		}

		var linkCount int64
		if err := conn.Model(&model.MaintainerProject{}).
			Where("maintainer_id = ? AND project_id = ?", maintainerID, projectID).
			Count(&linkCount).Error; err != nil {
			return nil, fmt.Errorf("check link maintainer=%d project=%d: %w", maintainerID, projectID, err)
		}
		if linkCount == 0 {
			continue
		}

		name := strings.TrimSpace(meta.FoundationCSVName)
		if name != "" && !strings.EqualFold(strings.TrimSpace(maintainer.Name), name) {
			seen[key] = true
			out = append(out, badLink{
				AuditLogID:   entry.ID,
				MaintainerID: maintainerID,
				ProjectID:    projectID,
				ProjectName:  meta.ProjectName,
				Tier:         tierProven,
				FoundName:    name,
				FoundCompany: meta.FoundationCompany,
				Maintainer:   *maintainer,
			})
			continue
		}

		// No recorded name to prove this one - defer to a live-file check.
		seen[key] = true
		unproven = append(unproven, badLink{
			AuditLogID:   entry.ID,
			MaintainerID: maintainerID,
			ProjectID:    projectID,
			ProjectName:  meta.ProjectName,
			Tier:         tierCorroborated,
			Maintainer:   *maintainer,
		})
	}

	if !liveCheck || len(unproven) == 0 {
		return out, nil
	}

	client := &http.Client{Timeout: 20 * time.Second}
	for _, l := range unproven {
		project, err := loadProject(l.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("load project %d: %w", l.ProjectID, err)
		}
		if project == nil {
			continue
		}
		ref := strings.TrimSpace(project.LegacyMaintainerRef)
		if ref == "" {
			log.Printf("skipping unproven link maintainer_id=%d project_id=%d: no maintainers file reference on project", l.MaintainerID, l.ProjectID)
			continue
		}
		body, err := fetchMaintainerRef(ctx, client, ref)
		if err != nil {
			log.Printf("skipping unproven link maintainer_id=%d project_id=%d: fetch error: %v", l.MaintainerID, l.ProjectID, err)
			continue
		}
		handle := strings.TrimSpace(strings.ToLower(l.Maintainer.GitHubAccount))
		name := strings.TrimSpace(l.Maintainer.Name)
		present := false
		if handle != "" && handle != "github_missing" && handlePresent(body, handle) {
			present = true
		} else if namePresent(body, name) {
			present = true
		}
		if present {
			continue // still genuinely listed in the current file - leave alone
		}
		out = append(out, l)
	}

	return out, nil
}

func severLink(conn *gorm.DB, l badLink) (bool, error) {
	tx := conn.Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	result := tx.Exec("DELETE FROM maintainer_projects WHERE maintainer_id = ? AND project_id = ?", l.MaintainerID, l.ProjectID)
	if result.Error != nil {
		tx.Rollback()
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return false, nil
	}

	maintainerID := l.MaintainerID
	projectID := l.ProjectID
	metadata, err := json.Marshal(map[string]any{
		"reason":                     "sentinel-collision bug (EMAIL_MISSING lookup matched an unrelated maintainer)",
		"evidence_tier":              string(l.Tier),
		"source_audit_log_id":        l.AuditLogID,
		"foundation_csv_name":        l.FoundName,
		"foundation_csv_company":     l.FoundCompany,
		"severed_from_maintainer_id": maintainerID,
	})
	if err != nil {
		tx.Rollback()
		return false, err
	}

	message := fmt.Sprintf("severed bad maintainer_projects link for maintainer %d / project %d (%s), evidence=%s",
		maintainerID, projectID, l.ProjectName, l.Tier)
	if l.FoundName != "" {
		message = fmt.Sprintf("severed bad maintainer_projects link for maintainer %d / project %d (%s): audit trail shows it belongs to %q, not %q",
			maintainerID, projectID, l.ProjectName, l.FoundName, l.Maintainer.Name)
	}

	event := model.AuditLog{
		ProjectID:    &projectID,
		MaintainerID: &maintainerID,
		Action:       "SEVER_SENTINEL_MAINTAINER_LINK",
		Message:      message,
		Metadata:     string(metadata),
	}
	if err := tx.Create(&event).Error; err != nil {
		tx.Rollback()
		return false, err
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

func fetchMaintainerRef(ctx context.Context, client *http.Client, urlStr string) (string, error) {
	raw, err := normalizeMaintainerRefURL(urlStr)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	// #nosec G704 -- URL is validated and allowlisted in normalizeMaintainerRefURL.
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func normalizeMaintainerRefURL(u string) (string, error) {
	raw := toRawGitHubURL(u)
	if !strings.HasPrefix(strings.ToLower(raw), "https://raw.githubusercontent.com/") {
		return "", fmt.Errorf("unsupported or non-allowlisted host in %q", u)
	}
	return raw, nil
}

func toRawGitHubURL(u string) string {
	if strings.Contains(u, "github.com") && strings.Contains(u, "/blob/") {
		parts := strings.Split(u, "github.com/")
		if len(parts) == 2 {
			rest := parts[1]
			rest = strings.Replace(rest, "/blob/", "/", 1)
			return "https://raw.githubusercontent.com/" + rest
		}
	}
	return u
}

// Matches GitHub handles with or without a leading @ (and github.com/ links).
var handleRE = regexp.MustCompile(`(?i)(?:github\.com/|@)?([a-z0-9](?:[a-z0-9-]{0,38}))`)

func handlePresent(body, handle string) bool {
	lower := strings.ToLower(body)
	handle = strings.ToLower(handle)
	matches := handleRE.FindAllStringSubmatch(lower, -1)
	for _, m := range matches {
		if len(m) > 1 && strings.EqualFold(m[1], handle) {
			return true
		}
	}
	return false
}

var spaceRE = regexp.MustCompile(`\s+`)

func namePresent(body, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	escaped := regexp.QuoteMeta(name)
	escaped = spaceRE.ReplaceAllString(escaped, `\s+`)
	re, err := regexp.Compile(`(?i)(^|[^a-z0-9_])` + escaped + `([^a-z0-9_]|$)`)
	if err != nil {
		return false
	}
	return re.FindStringIndex(body) != nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
