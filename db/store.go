package db

import (
	"maintainerd/model"
)

type Store interface {
	GetProjectsUsingService(serviceID uint) ([]model.Project, error)
	GetProjectMaintainersMap() (map[uint]model.Maintainer, error)
	GetMaintainersByProject(projectID uint) ([]model.Maintainer, error)
	GetServiceTeamByProject(projectID uint, serviceID uint) (*model.ServiceTeam, error)
	GetServiceIDByName(name string) (*model.Service, error)
	GetProjectServiceTeamMap(serviceID uint) (map[uint]*model.ServiceTeam, error)
	GetProjectIDMaintainersMap() (map[uint]model.ProjectInfo, error)
}
