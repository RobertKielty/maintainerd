package db

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"log"
	"maintainerd/model"
	"strings"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db}
}

// getServiceByName returns a &Service the service identified by name
func (s *SQLStore) getServiceByName(name string) (*model.Service, error) {
	var svc model.Service
	err := s.db.Where("name = ?", name).First(&svc).Error
	return &svc, err
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

// GetMaintainerMapByEmail returns a map of Maintainers keyed by email address
func (s *SQLStore) GetMaintainerMapByEmail() (map[string]model.Maintainer, error) {
	var maintainers []model.Maintainer
	err := s.db.Find(&maintainers).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]model.Maintainer)
	for _, maintainer := range maintainers {
		m[maintainer.Email] = maintainer
	}
	return m, nil
}

// GetMaintainerMapByGitHubAccount returns a map of Maintainers keyed by GitHub Account
func (s *SQLStore) GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error) {
	var maintainers []model.Maintainer
	err := s.db.Find(&maintainers).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]model.Maintainer)
	for _, maintainer := range maintainers {
		m[maintainer.GitHubAccount] = maintainer
	}
	return m, nil
}

// GetProjectMaintainersMap returns a map keyed by the project id which holds a list of Maintainers
// associated with that project.
func (s *SQLStore) GetProjectMaintainersMap() (map[uint]model.ProjectInfo, error) {
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
func (s *SQLStore) GetProjectServiceTeamMap(serviceName string) (map[uint]*model.ServiceTeam, error) {
	var serviceTeams []model.ServiceTeam
	service, err := s.getServiceByName(serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service, %s, by name: %v", serviceName, err)
	}
	// Preload the many-to-many relationship
	err = s.db.
		Where("service_id = ? ", service.ID).
		Find(&serviceTeams).Error
	if err != nil {
		return nil, fmt.Errorf("querying ServiceTeam for service_id %d: %w", service.ID, err)
	}

	result := make(map[uint]*model.ServiceTeam, len(serviceTeams))

	for i := range serviceTeams {
		st := &serviceTeams[i]
		result[st.ProjectID] = st
	}

	return result, nil

}
func (s *SQLStore) GetProjectMapByName() (map[string]model.Project, error) {
	var projects []model.Project
	if err := s.db.Find(&projects).Error; err != nil {
		return nil, err
	}

	projectsByName := make(map[string]model.Project)
	for _, p := range projects {
		projectsByName[p.Name] = p
	}
	return projectsByName, nil
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

func (s *SQLStore) LogAuditEvent(logger *zap.SugaredLogger, event model.AuditLog) error {
	if event.Message == "" {
		event.Message = event.Action
	}

	err := s.db.WithContext(context.Background()).Create(&event).Error
	if err != nil {
		logger.Errorf("failed to write audit log: %v", err)
		return err
	}

	logger.Infow("audit log recorded",
		"project_id", event.ProjectID,
		"maintainer_id", event.MaintainerID,
		"service_id", event.ServiceID,
		"action", event.Action,
		"message", event.Message,
	)
	return nil
}

// CreateServiceTeam creates or retrieves a service team entry in the database based on the provided project and service details.
// It accepts a project ID, project name, service ID, and service name as input and returns the service team or an error.
func (s *SQLStore) CreateServiceTeam(
	projectID uint, projectName string,
	serviceID int, serviceName string) (*model.ServiceTeam, error) {

	var errMessages []string

	st := &model.ServiceTeam{
		ServiceTeamID:   serviceID,
		ServiceID:       1, // TODO : Hardcoded to FOSSA for now
		ServiceTeamName: &serviceName,
		ProjectID:       projectID,
		ProjectName:     &projectName,
	}
	err := s.db.Where("service_team_id = ?", serviceID).FirstOrCreate(st).Error
	if err != nil {
		msg := fmt.Sprintf("CreateServiceTeamsForUser: failed for team %d (%s): %v", serviceID, serviceName, err)
		log.Println(msg)
		return nil, fmt.Errorf("CreateServiceTeamsForUser had partial errors:\n%s", strings.Join(errMessages, "\n"))
	}
	return st, nil
}
