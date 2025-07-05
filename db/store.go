package db

import (
	"go.uber.org/zap"
	"maintainerd/model"
)

type Store interface {
	GetProjectsUsingService(serviceID uint) ([]model.Project, error)
	GetProjectMaintainersMap() (map[uint]model.ProjectInfo, error)
	GetProjectMapByName() (map[string]model.Project, error)
	GetMaintainersByProject(projectID uint) ([]model.Maintainer, error)
	GetProjectServiceTeamMap(serviceName string) (map[uint]*model.ServiceTeam, error)
	GetProjectIDMaintainersMap() (map[uint]model.ProjectInfo, error)
	GetMaintainerMapByEmail() (map[string]model.Maintainer, error)
	GetServiceTeamByProject(projectID uint, serviceID uint) (*model.ServiceTeam, error)
	LogAuditEvent(logger *zap.SugaredLogger, event model.AuditLog) error
	GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error)
}
