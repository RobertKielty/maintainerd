package main

import (
	"context"
	"database/sql/driver"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"maintainerd/plugins/fossa"
)

type MaintainerStatus string

const (
	apiTokenEnvVar = "FOSSA_API_TOKEN"
)

const (
	ActiveMaintainer      MaintainerStatus = "Active"
	EmeritusMaintainer    MaintainerStatus = "Emeritus"
	RetiredMaintainer     MaintainerStatus = "Retired"
	StatusHdr             string           = "Status"
	ProjectHdr            string           = "Project"
	MaintNameHdr          string           = "Maintainer Name"
	CompanyNameHdr        string           = "Company"
	EmailHdr              string           = "Emails"
	GitHubHdr             string           = "Github Name"
	ParentProjectHdr      string           = "Parent Project"
	MaintainterFileRefHdr string           = "OWNERS/MAINTAINERS"
	MailingListAddrHdr    string           = "Mailing List Address"
)

// IsValid returns true id MaintainerStatus is known
func (s MaintainerStatus) IsValid() bool {
	switch s {
	case ActiveMaintainer, EmeritusMaintainer, RetiredMaintainer:
		return true
	}
	return false
}

func (s *MaintainerStatus) Scan(value interface{ any }) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into MaintainerStatus", value)
	}
	*s = MaintainerStatus(v)
	return nil
}

func (s MaintainerStatus) Value() (driver.Value, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("invalid MaintainerStatus %q", s)
	}
	return string(s), nil
}

// Maturity - A Project's maturity used by end-users to assess deployability
type Maturity string

const (
	Sandbox    Maturity = "Sandbox"
	Incubating Maturity = "Incubating"
	Graduated  Maturity = "Graduated"
	Archived   Maturity = "Archived"
)

func (m *Maturity) Scan(value interface{ any }) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into Maturity", value)
	}
	*m = Maturity(v)
	return nil
}

func (m Maturity) Value() (driver.Value, error) {
	if !m.IsValid() {
		return nil, fmt.Errorf("invalid Maturity %q", m)
	}
	return string(m), nil
}

func (m Maturity) IsValid() bool {
	switch m {
	case Sandbox, Incubating, Graduated, Archived:
		return true
	}
	return false
}

// A Maintainer is a leader that can speak for a Project
//
// At registration, an email needs to be provided
// Optionally, a Maintainer
//
//		has a Company Affiliation
//	  	TODO kubernetes specific may or may not have have voting rights on a Project,
//	    has a status of Active, Emeritus or Retired
type Maintainer struct {
	gorm.Model
	Name             string
	Email            string           `gorm:"size:254;default:EMAIL_MISSING"`
	GitHubAccount    string           `gorm:"size:100;default:GITHUB_MISSING"`
	MaintainerStatus MaintainerStatus `gorm:"type:text"`
	ImportWarnings   string
	Projects         []Project `gorm:"many2many:maintainer_projects;joinForeignKey:MaintainerID;joinReferences:ProjectID"`
	RegisteredAt     *time.Time
	CompanyID        *uint
	Company          Company
}

type Project struct {
	gorm.Model
	Name            string `gorm:"uniqueIndex,not null;check:name <> ''"`
	ParentProjectID *uint  `gorm:"index"`
	Maturity        Maturity
	MaintainerRef   string
	MailingList     *string      `gorm:"size:254;default:MML_MISSING"`
	Maintainers     []Maintainer `gorm:"many2many:maintainer_projects;joinForeignKey:ProjectID;joinReferences:MaintainerID"`
	Services        []Service    `gorm:"many2many:service_projects;joinForeignKey:ProjectID;joinReferences:ServiceID"`
}

type MaintainerProject struct {
	MaintainerID uint       `gorm:"primaryKey;index"` // FK + index
	ProjectID    uint       `gorm:"primaryKey;index"` // FK + index
	JoinedAt     time.Time  `gorm:"autoCreateTime"`
	Maintainer   Maintainer `gorm:"foreignKey:MaintainerID;constraint:OnDelete:CASCADE"`
	Project      Project    `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
}

type Company struct {
	gorm.Model
	Name string `gorm:"uniqueIndex"`
}

// For in memory cache
type ProjectInfo struct {
	Project     Project
	Maintainers []Maintainer
	Services    []Service
}

type Service struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex"`
	Description string
}

type ServiceTeam struct {
	gorm.Model
	ProjectID uint `gorm:"primaryKey;index"` // FK + index
	ServiceID uint `gorm:"primaryKey;index"` // FK + index
	RemoteID  int  // Service specific ID that identifies the project as a grouping on the service
}

type ServiceUser struct {
	gorm.Model
	ServiceID    uint  `gorm:"primaryKey;index"`
	MaintainerID *uint `gorm:"primaryKey;index"`
	CNCFAdminID  *uint `gorm:"primaryKey;index"`
}

// A FoundationOfficer is a person who has elevated access to
// Services to carry out Maintainer Operations on behalf of the
// Foundation that governs projects.
type FoundationOfficer struct {
	gorm.Model
	Name          string
	Email         string `gorm:"size:254;default:EMAIL_MISSING"`
	GitHubAccount string `gorm:"size:100;default:GITHUB_MISSING"`
	RegisteredAt  *time.Time
	CompanyID     *uint
	Services      []ServiceUser
}

type ReconciliationResult struct {
	gorm.Model
	Service              Service
	ProjectID            *uint
	MissingMaintainerIDs []*uint
}
type ServiceInterface interface {
	listRegisteredProjects() []string
	listRegisteredMaintainer(project string) []string
}

// Action describes steps taken during a reconciliation loop
type Action struct {
	Key         string
	ActionTaken string
	ServiceID   Service
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

	var services = []Service{
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
		&Company{},
		&Project{},
		&Maintainer{},
		&MaintainerProject{},
		&Service{},
		&ServiceTeam{},
		&ServiceUser{},
		//		&FoundationOfficer{},
	); err != nil {
		log.Fatalf("maintainerd: db: auto‑migration failed: %v", err)
	}

	var loadData bool = false
	if loadData {
		loadServices(db, services)
		loadMaintainersAndProjects(db, spreadsheetId, readRange)
		loadFOSSAProjects(db, Service{
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

	for projectID, svcTeam := range serviceTeamMap {
		fossaTeamMemberEmails, err := fossaClient.FetchTeamUserEmails(svcTeam.RemoteID)
		prj, _ := getProjectByID(db, projectID)
		log.Printf("Reconciling project\n\t\t    project: %s\n\t\t    FOSSATeamID: %d\n\t\t    fossaTeameamMemberEmails: %v\n\t\t    FetchTemUsers err :%v",
			projectIDMaintainersMap[projectID].Project.Name, svcTeam.RemoteID, fossaTeamMemberEmails, err)
		if err != nil {
			log.Printf("https://app.fossa.com/account/settings/organization/teams/%d %s", svcTeam.RemoteID, prj.Name)
		} else {
			log.Printf("\t\t    FOSSA USERS FOUND!!!%v\n", fossaTeamMemberEmails)
			maintainerSet := make(map[string]struct{})
			for _, pi := range projectIDMaintainersMap[projectID].Maintainers {
				maintainerSet[strings.ToLower(pi.Email)] = struct{}{}
			}
			log.Printf("                            MAINTAINERS %v\n", maintainerSet)
			fossaSet := make(map[string]struct{})
			for _, u := range fossaTeamMemberEmails {
				fossaSet[strings.ToLower(u)] = struct{}{}
			}
			if len(fossaSet) == len(projectIDMaintainersMap) {
				log.Printf("reconcileFOSSATeams %s : %d SIGNED UP FOSSA USERS!\n",
					projectIDMaintainersMap[projectID].Project.Name,
					len(fossaSet))

			} else {
				log.Printf("reconcileFOSSATeams %s : %d of %d MAINTAINERS !\n",
					projectIDMaintainersMap[projectID].Project.Name,
					len(fossaSet),
					len(projectIDMaintainersMap))
			}
			for email := range maintainerSet {
				if _, ok := fossaSet[email]; !ok {
					log.Printf("reconcileFOSSATeams for %s : SEND INVITE to %s\n",
						projectIDMaintainersMap[projectID].Project.Name,
						email)
					fossaClient.FetchUserInvitations()
				}
			}
			for email := range fossaSet {
				if _, ok := maintainerSet[email]; !ok {
					log.Printf("reconcileFOSSATeams for %s : UNREGISTERED Maintainer?%s\n", projectIDMaintainersMap[projectID].Project.Name, email)
				}
			}
		}

	}
}

// loadFOSSAProjects is called once at db start-up to retrieve all FOSSA Teams from FOSSA
// and match them up with their corresponding Foundation Projects
func loadFOSSAProjects(db *gorm.DB, s Service, token string) {
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
				serviceTeam := ServiceTeam{
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
func getProjectServiceTeamMap(db *gorm.DB, serviceID uint) (map[uint]*ServiceTeam, error) {
	var serviceTeams []ServiceTeam

	// Preload the many-to-many relationship
	err := db.
		Where("service_id = ? ", serviceID).
		Find(&serviceTeams).Error
	if err != nil {
		return nil, fmt.Errorf("querying ServiceTeam for service_id %d: %w", serviceID, err)
	}

	result := make(map[uint]*ServiceTeam, len(serviceTeams))

	for i := range serviceTeams {
		st := &serviceTeams[i]
		result[st.ProjectID] = st
	}

	return result, nil

}

// getProjectMaintainersMap returns a map keyed by the project name which holds a list of Maintainers
// associated with that project.
func getProjectMaintainersMap(db *gorm.DB) (map[string]ProjectInfo, error) {
	var projects []Project

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
	var projects []Project

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

func loadServices(db *gorm.DB, services []Service) {
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, service := range services {
			if err := tx.FirstOrCreate(&service, Service{Name: service.Name}).Error; err != nil {
				return fmt.Errorf("db: loadServices - failed to create service %v: error %v", service.Name, err)
			}
		}
		return nil
	}); err != nil {
		log.Printf("db: loadServices TX not committed, error: %v", err)
	}
}

// getServiceByName returns a &Service the service identified by name
func getServiceByName(db *gorm.DB, name string) (*Service, error) {
	var svc Service
	err := db.Where("name = ?", name).First(&svc).Error
	return &svc, err
}

// getProjectByID returns a &Project
func getProjectByID(db *gorm.DB, id uint) (*Project, error) {
	var prj Project
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

		var parent Project
		if parentName := row[ParentProjectHdr]; parentName != "" {
			parent = Project{}
			if err := db.Where("name = ?", parentName).
				First(&parent).Error; err != nil {
			}
		}
		currentMaintainerRef = row[MaintainterFileRefHdr]
		currentMailingList = row[MailingListAddrHdr]

		if err := db.Transaction(func(tx *gorm.DB) error {
			var project Project
			if parent.Name == "" {
				project = Project{
					Name:          row[ProjectHdr],
					Maturity:      Maturity(row[StatusHdr]),
					MaintainerRef: currentMaintainerRef,
					MailingList:   &currentMailingList,
				}
			} else {
				project = Project{
					Name:            row[ProjectHdr],
					Maturity:        parent.Maturity,
					MaintainerRef:   currentMaintainerRef,
					MailingList:     &currentMailingList,
					ParentProjectID: &parent.ID,
				}
			}
			if err := tx.FirstOrCreate(&project, Project{Name: project.Name}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on project %v: error %v", project, err)
			}
			if err := tx.FirstOrCreate(&project, Project{Name: project.Name}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on project %v: error %v", project, err)
			}
			company := Company{Name: company}
			if err := tx.FirstOrCreate(&company, Company{Name: company.Name}).Error; err != nil {
				return fmt.Errorf("maintainerd-backend: loadMaintainersAndProjects - failed calling FirstOrCreate on company %v: error %v", company, err)
			}
			maintainer := Maintainer{
				Name:             name,
				GitHubAccount:    github,
				Email:            email,
				CompanyID:        &company.ID,
				MaintainerStatus: ActiveMaintainer,
			}
			if err := tx.FirstOrCreate(&maintainer, Maintainer{Email: maintainer.Email}).Error; err != nil {
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
				// everything else: just use what’s there (or empty string)
				rowMap[h] = cellVal
			}
		}

		rows = append(rows, rowMap)
	}

	return rows, nil
}
