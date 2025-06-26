package db

import (
	"fmt"
	"maintainerd/model"

	"gorm.io/gorm"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) GetProjectsUsingService(serviceID uint) ([]model.Project, error) {
	var projects []model.Project
	err := s.db.
		Joins("JOIN service_teams st ON st.project_id = projects.id").
		Where("st.service_id = ?", serviceID).
		Preload("Maintainers.Company").
		Find(&projects).Error
	return projects, err
}

func (s *SQLStore) GetMaintainersByProject(projectID uint) ([]model.Maintainer, error) {
	var maintainers []model.Maintainer
	err := s.db.
		Joins("JOIN maintainer_projects mp ON mp.maintainer_id = maintainers.id").
		Where("mp.project_id = ?", projectID).
		Preload("Company").
		Find(&maintainers).Error
	return maintainers, err
}

func (s *SQLStore) GetServiceTeamByProject(projectID, serviceID uint) (*model.ServiceTeam, error) {
	var st model.ServiceTeam
	err := s.db.
		Where("project_id = ? AND service_id = ?", projectID, serviceID).
		First(&st).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &st, err
}

// GetProjectMaintainersMap returns a map keyed by the project name which holds a list of Maintainers
// associated with that project.
func (s *SQLStore) GetProjectMaintainersMap() (map[string]model.ProjectInfo, error) {
	var projects []model.Project

	// Preload the many-to-many relationship
	err := s.db.Preload("Maintainers").Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]model.ProjectInfo)

	for _, project := range projects {
		result[project.Name] = model.ProjectInfo{
			Project:     project,
			Maintainers: project.Maintainers,
			Services:    project.Services,
		}
	}

	return result, nil
}

func (s *SQLStore) getProjectIDMaintainersMap() (map[uint]model.ProjectInfo, error) {
	var projects []model.Project

	// Preload the many-to-many relationship
	err := s.db.Preload("Maintainers").Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]model.ProjectInfo)

	for _, project := range projects {
		result[project.ID] = model.ProjectInfo{
			Project:     project,
			Maintainers: project.Maintainers,
			Services:    project.Services,
		}
	}

	return result, nil
}

// getProjectMaintainersMap returns a map keyed by the project name which holds a list of Maintainers
// associated with that project.
func (s *SQLStore) getProjectMaintainersMap() (map[string]model.ProjectInfo, error) {
	var projects []model.Project

	// Preload the many-to-many relationship
	err := s.db.Preload("Maintainers").Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]model.ProjectInfo)

	for _, project := range projects {
		result[project.Name] = model.ProjectInfo{
			Project:     project,
			Maintainers: project.Maintainers,
			Services:    project.Services,
		}
	}

	return result, nil
}

// GetProjectServiceTeamMap returns a map of projectID to ServiceTeams
// for every Project that uses the service identified by serviceId
func (s *SQLStore) GetProjectServiceTeamMap(serviceID uint) (map[uint]*model.ServiceTeam, error) {
	var serviceTeams []model.ServiceTeam

	// Preload the many-to-many relationship
	err := s.db.
		Where("service_id = ? ", serviceID).
		Find(&serviceTeams).Error
	if err != nil {
		return nil, fmt.Errorf("querying ServiceTeam for service_id %d: %w", serviceID, err)
	}

	result := make(map[uint]*model.ServiceTeam, len(serviceTeams))

	for i := range serviceTeams {
		st := &serviceTeams[i]
		result[st.ProjectID] = st
	}

	return result, nil

}
func (s *SQLStore) GetProjectIDMaintainersMap() (map[uint]model.ProjectInfo, error) {
	var projects []model.Project

	// Preload the many-to-many relationship
	err := s.db.Preload("Maintainers").Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]model.ProjectInfo)

	for _, project := range projects {
		result[project.ID] = model.ProjectInfo{
			Project:     project,
			Maintainers: project.Maintainers,
			Services:    project.Services,
		}
	}

	return result, nil
}
