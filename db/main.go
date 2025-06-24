package main

import (
	"context"
	"fmt"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"strings"

	"maintainerd/model"
	"maintainerd/plugins/fossa"
)

const (
	apiTokenEnvVar = "FOSSA_API_TOKEN"
)

const (
	StatusHdr             string = "Status"
	ProjectHdr            string = "Project"
	MaintNameHdr          string = "Maintainer Name"
	CompanyNameHdr        string = "Company"
	EmailHdr              string = "Emails"
	GitHubHdr             string = "Github Name"
	ParentProjectHdr      string = "Parent Project"
	MaintainterFileRefHdr string = "OWNERS/MAINTAINERS"
	MailingListAddrHdr    string = "Mailing List Address"
)

// Maturity - A Project's maturity used by end-users to assess deployability

// ProjectInfo is an in-memory cache. TODO Review this
type ProjectInfo struct {
	Project     model.Project
	Maintainers []model.Maintainer
	Services    []model.Service
}

// Action describes steps taken during a reconciliation loop
type Action struct {
	Key         string
	ActionTaken string
	ServiceID   model.Service
}
type DiffEntry struct {
	Project string
	Missing []string
	Extra   []string
}

// Action DB maintenance headache, so probably will just write to a file??
// First iteration: for FOSSA Account and Team Membership
// key will just be email addr

func main() {

	spreadsheetId := os.Getenv("MD_WORKSHEET")
	if spreadsheetId == "" {
		log.Fatal("Environment variable MD_WORKSHEET is not set")
	}

	fossaToken := os.Getenv(apiTokenEnvVar)
	if fossaToken == "" {
		log.Fatalf("maintainerd: db: ERROR: Environment variable %s is not set", apiTokenEnvVar)
	}

	var services = []model.Service{
		{
			Name:        "FOSSA",
			Description: "Static code check we use to ensure 3rd Party License Policy",
		},
		{
			Name:        "Service Desk",
			Description: "Jira",
		},
		{
			Name:        "cncf.groups.io",
			Description: "Mailing list channels",
		},
		{
			Name:        "Snyk",
			Description: "Static code checker for 3rd Party License Policy monitoring and compliance",
		},
	}

	readRange := "Active!A1:J1639" // Adjust range as needed

	db, err := gorm.Open(sqlite.Open("demo.db"), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("maintainerd: db: failed opening database: %v", err)
	}

	// Auto‑migrate the schema. The order matters when foreign keys are
	// present, so we put parent tables first.
	if err := db.AutoMigrate(
		&model.Company{},
		&model.Project{},
		&model.Maintainer{},
		&model.MaintainerProject{},
		&model.Service{},
		&model.ServiceTeam{},
		&model.ServiceUser{},
		//		&FoundationOfficer{},
	); err != nil {
		log.Fatalf("maintainerd: db: auto‑migration failed: %v", err)
	}

	var loadData bool = false
	if loadData {
		loadServices(db, services)
		loadMaintainersAndProjects(db, spreadsheetId, readRange)
		loadFOSSAProjects(db, model.Service{
			Model: gorm.Model{ID: 1},
			Name:  "FOSSA",
		}, fossaToken)
		log.Println("maintainerd: db: INFO database successfully seeded with Services, Maintainers, Projects and FOSSA Teams in the sqlite db demo.db")
	} else {
		log.Println("maintainerd: db: WARNING no dataload performed, using existing database")
	}
	reconcileFOSSATeams(db)
}

// reconcileFOSSATeams checks each FOSSA Team read from the DB and diffs the list of FOSSA Team Users with the list of
// Maintainers we have stored in the db for that Project.
//
// This iteration will only report the following differences...
//  1. List project maintainers who have not been added to their Team on FOSSA
//  2. List user accounts on the FOSSA Team that are not registered as Maintainers (not a problem, sometimes a Project
//     will request that a contributor works on FOSSA for the project)
//
// TODO Refactor reconciler functions into their own component
func reconcileFOSSATeams(db *gorm.DB) {
	projectIDMaintainersMap, _ := getProjectIDMaintainersMap(db)
	fossaService, _ := getServiceByName(db, "FOSSA")
	serviceTeamMap, _ := getProjectServiceTeamMap(db, fossaService.ID)
	fossaClient := fossa.NewClient(os.Getenv(apiTokenEnvVar))

	if resp, err := fossaClient.FetchUserInvitations(); err != nil {
		log.Printf("fossa rec, FetchUserInvitations error %v\n", err)
	} else {
		log.Printf("fossa rec, FetchUserInvitations response is%v\n", resp)
	}

	for projectID, svcTeam := range serviceTeamMap {
		fossaTeamMemberEmails, err := fossaClient.FetchTeamUserEmails(svcTeam.RemoteID)
		if err != nil {
			log.Printf("fossa rec, %s ID-%d, ERR, ftme, https://app.fossa.com/account/settings/organization/teams/%d , %v",
				projectIDMaintainersMap[projectID].Project.Name, projectIDMaintainersMap[projectID].Project.ID, svcTeam.RemoteID, err)
		} else {

			maintainerSet := make(map[string]struct{})

			for _, pi := range projectIDMaintainersMap[projectID].Maintainers {
				maintainerSet[strings.ToLower(pi.Email)] = struct{}{}
			}
			fossaSet := make(map[string]struct{})
			for _, u := range fossaTeamMemberEmails {
				fossaSet[strings.ToLower(u)] = struct{}{}
			}
			if len(fossaSet) == len(projectIDMaintainersMap) {
				log.Printf("fossa rec, %s : %d signed up team members in FOSSA\n",
					projectIDMaintainersMap[projectID].Project.Name,
					len(fossaSet))
			} else {
				log.Printf("fossa rec, %s, ID-%d, registered maintainer count %d, fossa team members count %d\n",
					projectIDMaintainersMap[projectID].Project.Name,
					projectIDMaintainersMap[projectID].Project.ID,
					len(projectIDMaintainersMap[projectID].Maintainers),
					len(fossaSet))
			}
			log.Printf("fossa rec, %s, ID-%d, https://app.fossa.com/account/settings/organization/teams/%d , %v",
				projectIDMaintainersMap[projectID].Project.Name,
				projectIDMaintainersMap[projectID].Project.ID,
				svcTeam.RemoteID,
				fossaTeamMemberEmails)

			var invitations []string
			for email := range maintainerSet {
				if _, ok := fossaSet[email]; !ok {
					invitations = append(invitations, email)
				}
			}
			if len(invitations) > 0 {
				log.Printf("fossa rec, %s, ID-%d, send %d invites to : %v\n",
					projectIDMaintainersMap[projectID].Project.Name,
					projectIDMaintainersMap[projectID].Project.ID,
					len(invitations),
					invitations)
			}
			var unregisteredMembers []string
			for email := range fossaSet {
				if _, ok := maintainerSet[email]; !ok {
					unregisteredMembers = append(unregisteredMembers, email)
				}
			}
			if len(unregisteredMembers) > 0 {
				log.Printf("fossa rec, %s, ID-%d, unregistered FOSSA team members, %v\n",
					projectIDMaintainersMap[projectID].Project.Name,
					projectIDMaintainersMap[projectID].Project.ID,
					unregisteredMembers)
			}
		}

	}
}

func getMaintainerCountFromDb(db *gorm.DB, projectId uint) int64 {
	var count int64
	db.Model(&model.MaintainerProject{}).Where("PROJECT_ID = ?", projectId).Count(&count)
	return count
}

// loadFOSSAProjects is called once at db start-up to retrieve all FOSSA Teams from FOSSA
// and match them up with their corresponding Foundation Projects
func loadFOSSAProjects(db *gorm.DB, s model.Service, token string) {
	fossaClient := fossa.NewClient(token)
	pm, err := getProjectMaintainersMap(db)
	if err != nil {
		log.Printf("db, loadFOSSAProjects, getRegisteredMaintainers failed: %v", err)
	}
	ftm, err := fossaClient.FetchTeamsMap()
	log.Printf("loadFOSSAProjects, %d teams in fossaTeams, %d foundation projects\n", len(ftm), len(pm))
	log.Printf("loadFOSSAProjects, %v is the service\n", s)

	if err := db.Transaction(func(tx *gorm.DB) error {
		for fossaTeamName, fossaTeam := range ftm {
			if pm[fossaTeamName].Project.ID != 0 {
				log.Printf("project %s is on FOSSA ID is %d", pm[fossaTeamName].Project.Name, fossaTeam.ID)
				serviceTeam := model.ServiceTeam{
					ProjectID: pm[fossaTeamName].Project.ID,
					ServiceID: 1,
					RemoteID:  fossaTeam.ID,
				}
				if err := tx.FirstOrCreate(&serviceTeam).Error; err != nil {
					return fmt.Errorf("db: loadFOSSAProjects - ERROR adding FOSSATeam %s to Project %s : %v",
						fossaTeamName, pm[fossaTeamName].Project.Name, err)
				}
			}
		}
		return nil
	}); err != nil {
		log.Printf("db, loadFOSSAProjects, TX failed: %v", err)
	}
	return
}

// getProjectServicesTeamMap returns a map keyed by the project ID whose value is an array ServiceTeams
func getProjectServiceTeamMap(db *gorm.DB, serviceID uint) (map[uint]*model.ServiceTeam, error) {
	var serviceTeams []model.ServiceTeam

	// Preload the many-to-many relationship
	err := db.
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

// getProjectMaintainersMap returns a map keyed by the project name which holds a list of Maintainers
// associated with that project.
func getProjectMaintainersMap(db *gorm.DB) (map[string]ProjectInfo, error) {
	var projects []model.Project

	// Preload the many-to-many relationship
	err := db.Preload("Maintainers").Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]ProjectInfo)

	for _, project := range projects {
		result[project.Name] = ProjectInfo{
			Project:     project,
			Maintainers: project.Maintainers,
			Services:    project.Services,
		}
	}

	return result, nil
}

func getProjectIDMaintainersMap(db *gorm.DB) (map[uint]ProjectInfo, error) {
	var projects []model.Project

	// Preload the many-to-many relationship
	err := db.Preload("Maintainers").Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]ProjectInfo)

	for _, project := range projects {
		result[project.ID] = ProjectInfo{
			Project:     project,
			Maintainers: project.Maintainers,
			Services:    project.Services,
		}
	}

	return result, nil
}

// loadServiceTeam reaches out to a Service to see if the Project is registered with the service
// if it is registered, then we add a Reference to the specific "GroupContainer" allocated to the Project.
// The term "Group Container" is an attempt to smooth over the variety of terms that are used to
// describe where on the Service a set of our Project Maintainers are added.
func loadServices(db *gorm.DB, services []model.Service) {
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, service := range services {
			if err := tx.FirstOrCreate(&service, model.Service{Name: service.Name}).Error; err != nil {
				return fmt.Errorf("db: loadServices - failed to create service %v: error %v", service.Name, err)
			}
		}
		return nil
	}); err != nil {
		log.Printf("db: loadServices TX not committed, error: %v", err)
	}
}

// getServiceByName returns a &Service the service identified by name
func getServiceByName(db *gorm.DB, name string) (*model.Service, error) {
	var svc model.Service
	err := db.Where("name = ?", name).First(&svc).Error
	return &svc, err
}

// getProjectByID returns a &Project
func getProjectByID(db *gorm.DB, id uint) (*model.Project, error) {
	var prj model.Project
	err := db.Where("id = ?", id).First(&prj).Error
	return &prj, err
}

// Reads the readRange data from spreadsheetID inserts it into db
// The readRange from the worksheet MUST include the header row
func loadMaintainersAndProjects(db *gorm.DB, spreadsheetID, readRange string) {
	ctx := context.Background()

	srv, err := sheets.NewService(
		ctx,
		option.WithCredentialsFile("credentials.json"),
		option.WithScopes(sheets.SpreadsheetsReadonlyScope),
	)

	if err != nil {
		log.Fatalf("maintainerd: backend: loadMaintainersAndProjects: unable to retrieve Sheets client: %v", err)
	}

	rows, err := readSheetRows(ctx, srv, spreadsheetID, readRange)

	if err != nil {
		log.Fatalf("maintainerd-backend: loadMaintainersAndProjects - readSheetRows: %v", err)
	}

	var currentMaintainerRef string
	var currentMailingList string

	for _, row := range rows {
		var missingMaintainerFields []string

		name := row[MaintNameHdr]

		if name == "" {
			missingMaintainerFields = append(missingMaintainerFields, ":"+MaintNameHdr)
		}

		company := row[CompanyNameHdr]
		if company == "" {
			missingMaintainerFields = append(missingMaintainerFields, ":"+CompanyNameHdr)
		}

		email := row[EmailHdr]
		if email == "" {
			missingMaintainerFields = append(missingMaintainerFields, ":"+EmailHdr)
		}

		github := row[GitHubHdr]
		if github == "" {
			missingMaintainerFields = append(missingMaintainerFields, ":"+GitHubHdr)
		}

		var parent model.Project
		if parentName := row[ParentProjectHdr]; parentName != "" {
			parent = model.Project{}
			if err := db.Where("name = ?", parentName).
				First(&parent).Error; err != nil {
			}
		}
		currentMaintainerRef = row[MaintainterFileRefHdr]
		currentMailingList = row[MailingListAddrHdr]

		if err := db.Transaction(func(tx *gorm.DB) error {
			var project model.Project
			if parent.Name == "" {
				project = model.Project{
					Name:          row[ProjectHdr],
					Maturity:      model.Maturity(row[StatusHdr]),
					MaintainerRef: currentMaintainerRef,
					MailingList:   &currentMailingList,
				}
			} else {
				project = model.Project{
					Name:            row[ProjectHdr],
					Maturity:        parent.Maturity,
					MaintainerRef:   currentMaintainerRef,
					MailingList:     &currentMailingList,
					ParentProjectID: &parent.ID,
				}
			}
			if err := tx.FirstOrCreate(&project, model.Project{Name: project.Name}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on project %v: error %v", project, err)
			}
			if err := tx.FirstOrCreate(&project, model.Project{Name: project.Name}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on project %v: error %v", project, err)
			}
			company := model.Company{Name: company}
			if err := tx.FirstOrCreate(&company, model.Company{Name: company.Name}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on company %v: error %v", company, err)
			}
			maintainer := model.Maintainer{
				Name:             name,
				GitHubAccount:    github,
				Email:            email,
				CompanyID:        &company.ID,
				MaintainerStatus: model.ActiveMaintainer,
			}
			if err := tx.FirstOrCreate(&maintainer, model.Maintainer{Email: maintainer.Email}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on maintainer %v: error %v", maintainer, err)
			}
			// Ensure the association (in case the maintainer existed already)
			return tx.Model(&maintainer).
				Association("Projects").
				Append(&project)
		}); err != nil {
			log.Printf("TX not committed, row skipped %v : error %v ", row, err)
		}
	}
}

// readSheetRows returns every row as a map keyed by the header row and carries forward the last non‐empty Project and
// Status values when those cells are blank or missing.
func readSheetRows(ctx context.Context, srv *sheets.Service, spreadsheetID, readRange string) ([]map[string]string, error) {
	resp, err := srv.Spreadsheets.Values.
		Get(spreadsheetID, readRange).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("db: unable to retrieve worksheet data: %w", err)
	}
	if len(resp.Values) == 0 {
		return nil, fmt.Errorf("db: worksheet is empty")
	}

	// First row → headers
	headers := make([]string, len(resp.Values[0]))
	for i, cell := range resp.Values[0] {
		headers[i] = strings.TrimSpace(fmt.Sprint(cell))
	}

	// Find the column indexes for "Project" and "Status"
	projIdx, statIdx := -1, -1
	for i, h := range headers {
		switch h {
		case "Project":
			projIdx = i
		case "Status":
			statIdx = i
		}
	}

	var rows []map[string]string
	var lastProject, lastStatus string

	// Remaining rows → maps
	for _, r := range resp.Values[1:] {
		rowMap := make(map[string]string, len(headers))

		for i, h := range headers {
			// read raw cell if present
			var cellVal string
			if i < len(r) {
				cellVal = strings.TrimSpace(fmt.Sprint(r[i]))
			}

			switch i {
			case projIdx:
				if cellVal != "" {
					lastProject = cellVal
				}
				rowMap[h] = lastProject

			case statIdx:
				if cellVal != "" {
					lastStatus = cellVal
				}
				rowMap[h] = lastStatus

			default:
				rowMap[h] = cellVal
			}
		}

		rows = append(rows, rowMap)
	}

	return rows, nil
}
