package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"maintainerd/db"
	"maintainerd/model"
	"maintainerd/plugins/fossa"

	"go.uber.org/zap"

	"gorm.io/gorm"
)

const defaultDBPath = "/data/maintainers.db"

func main() {
	logger := log.New(os.Stdout, "fossa-poller: ", log.LstdFlags)

	interval := flag.Duration("interval", 5*time.Minute, "polling interval (set <=0 to run once)")
	flag.Parse()

	dbDriver := envOr("MD_DB_DRIVER", "sqlite")
	dbDSN := envOr("MD_DB_DSN", "")
	dbPath := envOr("MD_DB_PATH", defaultDBPath)
	fossaToken := strings.TrimSpace(os.Getenv("FOSSA_API_TOKEN"))

	if dbDriver == "postgres" && dbDSN == "" {
		logger.Fatal("MD_DB_DSN is required when MD_DB_DRIVER=postgres")
	}
	if fossaToken == "" {
		logger.Fatal("FOSSA_API_TOKEN is required")
	}

	dsn := dbPath
	if dbDriver == "postgres" {
		dsn = dbDSN
	}

	dbConn, err := db.OpenGorm(dbDriver, dsn, &gorm.Config{})
	if err != nil {
		logger.Fatalf("failed to open DB: %v", err)
	}
	store := db.NewSQLStore(dbConn)

	client := fossa.NewClient(fossaToken)

	runOnce := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := pollFossaInvites(ctx, logger, store, client); err != nil {
			logger.Printf("poll failed: %v", err)
			return
		}
		logger.Println("poll complete")
	}

	if *interval <= 0 {
		runOnce()
		return
	}

	logger.Printf("starting poll loop interval=%s", interval.String())
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	runOnce()
	for range ticker.C {
		runOnce()
	}
}

type fossaAPI interface {
	HasPendingInvitation(email string) (bool, error)
	SendUserInvitation(email string) error
	AddUserToTeamByEmail(teamID uint, email string, roleID int) error
	AddUserToTeamByEmailWithResponse(teamID uint, email string, roleID int) (*fossa.TeamAddResponse, error)
	FetchTeamUserEmails(teamID uint) ([]string, error)
	FetchTeamMembersRaw(teamID uint) (fossa.TeamMembers, []byte, error)
	FindUserIDByEmail(email string) (uint, error)
	ResolveTeamAdminRoleID() (int, error)
}

func pollFossaInvites(ctx context.Context, logger *log.Logger, store *db.SQLStore, client fossaAPI) error {
	_ = ctx
	serviceID, err := getFossaServiceID(store)
	if err != nil {
		return err
	}
	pollerStaffID := ensurePollerStaff(store, logger)
	auditLogger := zap.NewNop().Sugar()
	dbInvites, err := store.ListServiceInvitationsByStatus(serviceID, []string{"pending", "expired", "accepted"})
	if err != nil {
		return err
	}
	if len(dbInvites) == 0 {
		logger.Println("no pending invites found in maintainer-d database")
		return nil
	}

	now := time.Now().UTC()
	const inviteTTL = 72 * time.Hour
	for _, invite := range dbInvites {
		email := strings.TrimSpace(invite.ServiceEmail)
		if email == "" || strings.EqualFold(email, "EMAIL_MISSING") {
			msg := "missing email address"
			invite.Status = "error"
			invite.LastError = &msg
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}
		if invite.RemoteTeamID == 0 {
			msg := "missing FOSSA team ID"
			invite.Status = "error"
			invite.LastError = &msg
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		if invite.SentAt != nil && now.Sub(*invite.SentAt) > inviteTTL {
			if err := client.SendUserInvitation(email); err != nil && !errors.Is(err, fossa.ErrInviteAlreadyExists) {
				msg := err.Error()
				invite.Status = "expired"
				invite.LastError = &msg
				invite.LastCheckedAt = &now
				if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
					logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
				}
				continue
			}
			invite.Status = "pending"
			invite.LastError = nil
			invite.SentAt = &now
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		logger.Printf("HasPendingInvitation %s", formatInviteSummary(store, invite))
		pending, err := client.HasPendingInvitation(email)
		if err != nil {
			msg := err.Error()
			invite.Status = "error"
			invite.LastError = &msg
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		if pending {
			invite.Status = "pending"
			invite.TeamAssignmentStatus = nil
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		if invite.TeamAssignmentStatus != nil && *invite.TeamAssignmentStatus == "done" {
			invite.Status = "accepted"
			invite.LastError = nil
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		if invite.TeamAssignmentStatus != nil && *invite.TeamAssignmentStatus == "error" {
			invite.Status = "accepted"
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		if invite.NextTeamAddAt != nil && now.Before(*invite.NextTeamAddAt) {
			invite.Status = "accepted"
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}

		teamAdminRoleID, err := client.ResolveTeamAdminRoleID()
		if err != nil {
			msg := err.Error()
			invite.Status = "error"
			invite.LastError = &msg
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}
		addResp, err := client.AddUserToTeamByEmailWithResponse(invite.RemoteTeamID, email, teamAdminRoleID)
		if err != nil {
			if errors.Is(err, fossa.ErrUserAlreadyMember) {
				invite.Status = "accepted"
				done := "done"
				invite.TeamAssignmentStatus = &done
				invite.TeamAddAttempts = 0
				invite.NextTeamAddAt = nil
				invite.LastError = nil
				invite.LastCheckedAt = &now
				if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
					logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
				}
				if recordErr := recordFossaTeamMembership(store, client, serviceID, invite); recordErr != nil {
					logger.Printf("record FOSSA team membership failed %s err=%v", formatInviteSummary(store, invite), recordErr)
				}
				logInviteAudit(store, auditLogger, "FOSSA_TEAM_MEMBER_ADDED", pollerStaffID, serviceID, invite, addResp, nil)
				continue
			}
			if errors.Is(err, fossa.ErrUserNotFound) {
				if err := client.SendUserInvitation(email); err != nil && !errors.Is(err, fossa.ErrInviteAlreadyExists) {
					msg := err.Error()
					invite.Status = "error"
					invite.LastError = &msg
					invite.LastCheckedAt = &now
					if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
						logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
					}
					continue
				}
				invite.Status = "pending"
				invite.LastError = nil
				invite.SentAt = &now
				invite.LastCheckedAt = &now
				if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
					logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
				}
				continue
			}
			msg := err.Error()
			invite.Status = "error"
			invite.LastError = &msg
			errorStatus := "error"
			invite.TeamAssignmentStatus = &errorStatus
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			logInviteAudit(store, auditLogger, "FOSSA_TEAM_MEMBER_ADD_FAILED", pollerStaffID, serviceID, invite, addResp, nil)
			continue
		}

		emails, fetchErr := client.FetchTeamUserEmails(invite.RemoteTeamID)
		_, teamMembersBody, teamMembersErr := client.FetchTeamMembersRaw(invite.RemoteTeamID)
		if teamMembersErr != nil {
			logger.Printf("verify FOSSA team membership raw fetch failed %s err=%v", formatInviteSummary(store, invite), teamMembersErr)
		}
		if fetchErr != nil {
			logger.Printf("verify FOSSA team membership failed %s err=%v", formatInviteSummary(store, invite), fetchErr)
		}
		if fetchErr == nil && !emailInList(email, emails) {
			logger.Printf("verify FOSSA team membership missing %s err=not_found_on_team", formatInviteSummary(store, invite))
			pendingStatus := "pending"
			invite.TeamAssignmentStatus = &pendingStatus
			invite.TeamAddAttempts++
			invite.NextTeamAddAt = nextTeamAddAt(now, invite.TeamAddAttempts)
			if invite.TeamAddAttempts >= 3 {
				errorStatus := "error"
				msg := "team assignment failed after 3 attempts"
				invite.TeamAssignmentStatus = &errorStatus
				invite.LastError = &msg
				invite.NextTeamAddAt = nil
				logInviteAudit(store, auditLogger, "FOSSA_TEAM_MEMBER_ADD_FAILED", pollerStaffID, serviceID, invite, addResp, teamMembersBody)
			}
			invite.Status = "accepted"
			invite.LastCheckedAt = &now
			if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
				logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
			}
			continue
		}
		if fetchErr == nil {
			logger.Printf("verify FOSSA team membership ok %s", formatInviteSummary(store, invite))
		}

		invite.Status = "accepted"
		done := "done"
		invite.TeamAssignmentStatus = &done
		invite.TeamAddAttempts = 0
		invite.NextTeamAddAt = nil
		invite.LastError = nil
		invite.LastCheckedAt = &now
		if _, upsertErr := store.UpsertServiceInvitation(&invite); upsertErr != nil {
			logger.Printf("upsert invite failed %s err=%v", formatInviteSummary(store, invite), upsertErr)
		}
		if recordErr := recordFossaTeamMembership(store, client, serviceID, invite); recordErr != nil {
			logger.Printf("record FOSSA team membership failed %s err=%v", formatInviteSummary(store, invite), recordErr)
		}
		logger.Printf("added %s to remoteTeamID=%d (%s)", email, invite.RemoteTeamID, formatInviteSummary(store, invite))
		logInviteAudit(store, auditLogger, "FOSSA_TEAM_MEMBER_ADDED", pollerStaffID, serviceID, invite, addResp, teamMembersBody)
	}
	return nil
}

func emailInList(target string, emails []string) bool {
	target = normalizeEmail(target)
	if target == "" {
		return false
	}
	for _, email := range emails {
		if normalizeEmail(email) == target {
			return true
		}
	}
	return false
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nextTeamAddAt(now time.Time, attempts int) *time.Time {
	if attempts <= 0 {
		return nil
	}
	backoff := 20 * time.Second
	next := now.Add(backoff)
	return &next
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func ensurePollerStaff(store *db.SQLStore, logger *log.Logger) *uint {
	const pollerStaff = "FOSSA_POLLER"
	var staff model.StaffMember
	if err := store.DB().Where("git_hub_account = ?", pollerStaff).First(&staff).Error; err == nil {
		return &staff.ID
	}
	staff = model.StaffMember{
		Name:          pollerStaff,
		GitHubAccount: pollerStaff,
		Email:         "EMAIL_MISSING",
		GitHubEmail:   "GITHUB_EMAIL_MISSING",
	}
	if err := store.DB().Create(&staff).Error; err != nil {
		logger.Printf("poller staff create failed err=%v", err)
		return nil
	}
	return &staff.ID
}

func logInviteAudit(store *db.SQLStore, logger *zap.SugaredLogger, action string, staffID *uint, serviceID uint, invite model.ServiceInvitation, addResp *fossa.TeamAddResponse, teamMembersBody []byte) {
	meta := map[string]json.RawMessage{}
	if addResp != nil {
		meta["add_user_response"] = rawOrWrapped(addResp.Body)
	}
	if len(teamMembersBody) > 0 {
		meta["team_members_response"] = rawOrWrapped(teamMembersBody)
	}
	context := map[string]interface{}{
		"remote_team_id": invite.RemoteTeamID,
		"email":          invite.ServiceEmail,
		"role_id":        addRespRoleID(addResp),
		"remote_user_id": addRespUserID(addResp),
	}
	contextBytes, err := json.Marshal(context)
	if err != nil {
		contextBytes = []byte("{}")
	}
	meta["context"] = rawOrWrapped(contextBytes)

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		metaBytes = []byte("{}")
	}
	message := fmt.Sprintf("FOSSA_POLLER: %s", invite.ServiceEmail)
	event := model.AuditLog{
		Action:       action,
		Message:      message,
		Metadata:     string(metaBytes),
		ProjectID:    &invite.ProjectID,
		MaintainerID: invite.MaintainerID,
		ServiceID:    &serviceID,
		StaffID:      staffID,
	}
	if err := store.LogAuditEvent(logger, event); err != nil {
		logger.Debugf("poller audit log failed action=%s err=%v", action, err)
	}
}

func rawOrWrapped(body []byte) json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	if json.Valid(body) {
		var indented bytes.Buffer
		if err := json.Indent(&indented, body, "", "  "); err == nil {
			return indented.Bytes()
		}
		return body
	}
	wrapped, err := json.Marshal(map[string]string{"raw": string(body)})
	if err != nil {
		return nil
	}
	return wrapped
}

func addRespRoleID(resp *fossa.TeamAddResponse) int {
	if resp == nil {
		return 0
	}
	return resp.RoleID
}

func addRespUserID(resp *fossa.TeamAddResponse) uint {
	if resp == nil {
		return 0
	}
	return resp.UserID
}

func getFossaServiceID(store *db.SQLStore) (uint, error) {
	var service model.Service
	if err := store.DB().Where("name = ?", "FOSSA").First(&service).Error; err != nil {
		return 0, err
	}
	return service.ID, nil
}

func recordFossaTeamMembership(store *db.SQLStore, client fossaAPI, serviceID uint, invite model.ServiceInvitation) error {
	email := strings.TrimSpace(invite.ServiceEmail)
	if email == "" || strings.EqualFold(email, "EMAIL_MISSING") {
		return fmt.Errorf("missing email")
	}
	userID, err := client.FindUserIDByEmail(email)
	if err != nil {
		return err
	}
	serviceTeam, err := store.GetRemoteTeamByProject(invite.ProjectID, serviceID)
	if err != nil {
		return err
	}
	if serviceTeam == nil {
		return fmt.Errorf("missing service team for project %d", invite.ProjectID)
	}
	remoteUser, err := store.UpsertRemoteUser(&model.RemoteUser{
		ServiceID:    serviceID,
		RemoteUserID: userID,
		ServiceEmail: email,
	})
	if err != nil {
		return err
	}
	link := &model.RemoteTeamUser{
		ServiceID:    serviceID,
		TeamID:       serviceTeam.ID,
		UserID:       remoteUser.ID,
		MaintainerID: invite.MaintainerID,
	}
	if _, err := store.UpsertRemoteUserTeam(link); err != nil {
		return err
	}
	return nil
}

func formatInviteSummary(store *db.SQLStore, invite model.ServiceInvitation) string {
	projectName := "unknown-project"
	if invite.ProjectID != 0 {
		var project model.Project
		if err := store.DB().Select("id", "name").First(&project, invite.ProjectID).Error; err == nil {
			if strings.TrimSpace(project.Name) != "" {
				projectName = project.Name
			}
		}
	}
	maintainerName := "unknown-maintainer"
	maintainerGitHub := "unknown-github"
	if invite.MaintainerID != nil && *invite.MaintainerID != 0 {
		var maintainer model.Maintainer
		if err := store.DB().Select("id", "name", "git_hub_account").First(&maintainer, *invite.MaintainerID).Error; err == nil {
			if strings.TrimSpace(maintainer.Name) != "" {
				maintainerName = maintainer.Name
			}
			if strings.TrimSpace(maintainer.GitHubAccount) != "" {
				maintainerGitHub = maintainer.GitHubAccount
			}
		}
	}
	sentAt := "unknown"
	ttl := "unknown"
	if invite.SentAt != nil {
		elapsed := time.Since(invite.SentAt.UTC())
		if elapsed < 0 {
			elapsed = 0
		}
		if elapsed < 60*time.Minute {
			mins := int(elapsed.Minutes())
			sentAt = "Sent " + strconv.Itoa(mins) + "m ago"
		} else {
			hours := int(elapsed.Hours())
			sentAt = "Sent " + strconv.Itoa(hours) + "h ago"
		}
		expiresAt := invite.SentAt.UTC().Add(72 * time.Hour)
		remaining := time.Until(expiresAt)
		if remaining < 0 {
			remaining = 0
		}
		if remaining < 60*time.Minute {
			mins := int(remaining.Minutes())
			ttl = "Expires in " + strconv.Itoa(mins) + "m"
		} else {
			hours := int(remaining.Hours())
			ttl = "Expires in " + strconv.Itoa(hours) + "h"
		}
	}
	return maintainerName +
		", " + projectName +
		", " + invite.ServiceEmail +
		", " + maintainerGitHub +
		", " + sentAt +
		", " + ttl
}
