package db

import (
	"errors"

	"maintainerd/model"

	"go.uber.org/zap"
)

var ErrProjectNotFound = errors.New("project not found")
var ErrProjectExists = errors.New("project already exists")
var ErrCompanyExists = errors.New("company already exists")

type Store interface {
	GetProjectsUsingService(serviceID uint) ([]model.Project, error)
	ListProjects() ([]model.Project, error)
	GetProjectByID(projectID uint) (*model.Project, error)
	GetProjectMapByName() (map[string]model.Project, error)
	ListMaintainers() ([]model.Maintainer, error)
	ListMaintainersWithoutIdentityObservation(source string) ([]model.Maintainer, error)
	GetMaintainersByProject(projectID uint) ([]model.Maintainer, error)
	GetProjectRemoteTeamMap(serviceName string) (map[uint]*model.RemoteTeam, error)
	GetMaintainerMapByEmail() (map[string]model.Maintainer, error)
	GetRemoteTeamByProject(projectID uint, serviceID uint) (*model.RemoteTeam, error)
	LogAuditEvent(logger *zap.SugaredLogger, event model.AuditLog) error
	GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error)
	CreateRemoteTeam(projectID uint, projectName string, serviceID uint, remoteTeamID uint, remoteTeamName string) (*model.RemoteTeam, error)
	UpsertMaintainer(projectID uint, name, email, githubHandle, company string) (*model.Maintainer, error)
	UpsertMaintainerWithIdentity(projectID uint, name, email, githubHandle, company, lfxUserID string) (*model.Maintainer, bool, bool, error)
	CreateCompany(name string) (*model.Company, error)
	UpdateProjectMaturity(projectID uint, maturity model.Maturity) error
	UpdateProjectLegacyMaintainerRef(projectID uint, ref string) error
	UpdateProjectDotProjectMetadata(projectID uint, patch model.Project) error
	PersistDotProjectSync(projectID uint, patch model.Project, state *model.DotProjectSyncState) error
	UpdateMaintainerStatus(maintainerID uint, status model.MaintainerStatus) error
	UpdateMaintainersStatus(ids []uint, status model.MaintainerStatus) error
	UpdateMaintainerGitHubEmail(maintainerID uint, githubEmail string) error
	UpdateMaintainerDetails(maintainerID uint, name, email, github, githubEmail string, location *string, status model.MaintainerStatus, companyID *uint) (*model.Maintainer, error)
	ListCompanies() ([]model.Company, error)
	ListStaffMembers() ([]model.StaffMember, error)
	ListServiceInvitations(projectID uint, serviceID uint) ([]model.ServiceInvitation, error)
	ListServiceInvitationsByStatus(serviceID uint, statuses []string) ([]model.ServiceInvitation, error)
	UpsertServiceInvitation(invite *model.ServiceInvitation) (*model.ServiceInvitation, error)
	GetServiceInvitationByID(id uint) (*model.ServiceInvitation, error)
	DeleteServiceInvitation(id uint) error
	ListRemoteTeamMaintainers(teamID uint) ([]model.Maintainer, error)
	UpsertRemoteUser(user *model.RemoteUser) (*model.RemoteUser, error)
	UpsertRemoteUserTeam(link *model.RemoteTeamUser) (*model.RemoteTeamUser, error)
	GetMaintainerRefCache(projectID uint) (*model.MaintainerRefCache, error)
	UpsertMaintainerRefCache(cache *model.MaintainerRefCache) error
	GetDotProjectSyncState(projectID uint) (*model.DotProjectSyncState, error)
	UpsertDotProjectSyncState(state *model.DotProjectSyncState) error
	UpsertMaintainerIdentityObservation(observation *model.MaintainerIdentityObservation) (*model.MaintainerIdentityObservation, error)
	GetLatestMaintainerIdentityObservation(source string, maintainerID uint) (*model.MaintainerIdentityObservation, error)
	GetLatestMaintainerIdentityObservationByRef(source string, projectID uint, sourceRef string) (*model.MaintainerIdentityObservation, error)
	MergeCompanies(fromID, toID uint) error
}
