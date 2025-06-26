package db

import (
	"context"
	"fmt"
	"log"
	"strings"

	"maintainerd/model"
	"maintainerd/plugins/fossa"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	StatusHdr            string = "Status"
	ProjectHdr           string = "Project"
	MaintainerNameHdr    string = "Maintainer Name"
	CompanyNameHdr       string = "Company"
	EmailHdr             string = "Emails"
	GitHubHdr            string = "Github Name"
	ParentProjectHdr     string = "Parent Project"
	MaintainerFileRefHdr string = "OWNERS/MAINTAINERS"
	MailingListAddrHdr   string = "Mailing List Address"
)

func BootstrapSQLite(dbPath, spreadsheetID, readRange, worksheetCredentialsPath, fossaToken string, seed bool) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}

	if err := db.AutoMigrate(
		&model.Company{},
		&model.Project{},
		&model.Maintainer{},
		&model.MaintainerProject{},
		&model.Service{},
		&model.ServiceTeam{},
		&model.ServiceUser{},
	); err != nil {
		return nil, fmt.Errorf("auto-migration failed: %w", err)
	}

	if !seed {
		log.Println("bootstrap: database schema created but no seed data loaded")
		return db, nil
	}

	services := []model.Service{
		{Name: "FOSSA", Description: "Static code check we use to ensure 3rd Party License Policy"},
		{Name: "Service Desk", Description: "Jira"},
		{Name: "cncf.groups.io", Description: "Mailing list channels"},
		{Name: "Snyk", Description: "Static code checker for 3rd Party License Policy monitoring and compliance"},
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, service := range services {
			if err := tx.FirstOrCreate(&service, model.Service{Name: service.Name}).Error; err != nil {
				return fmt.Errorf("bootstrap: failed to insert service %s: %w", service.Name, err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := loadMaintainersAndProjects(db, spreadsheetID, readRange, worksheetCredentialsPath); err != nil {
		return nil, fmt.Errorf("bootstrap: failed to load maintainers and projects: %w", err)
	}

	fossaService := model.Service{Model: gorm.Model{ID: 1}, Name: "FOSSA"}
	if err := loadFOSSAProjects(db, fossaService, fossaToken); err != nil {
		return nil, fmt.Errorf("bootstrap: failed to load FOSSA projects: %w", err)
	}

	log.Printf("bootstrap: completed and loaded seed data into %s", dbPath)
	return db, nil
}

// Reads the readRange data from spreadsheetID inserts it into db
// The readRange from the worksheet MUST include the header row
func loadMaintainersAndProjects(db *gorm.DB, spreadsheetID, readRange, credentialsPath string) error {
	ctx := context.Background()

	srv, err := sheets.NewService(
		ctx,
		option.WithCredentialsFile(credentialsPath),
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

		name := row[MaintainerNameHdr]

		if name == "" {
			missingMaintainerFields = append(missingMaintainerFields, ":"+MaintainerNameHdr)
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
		currentMaintainerRef = row[MaintainerFileRefHdr]
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
	return nil
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

// loadFOSSAProjects is called once at db start-up to retrieve all FOSSA Teams from FOSSA
// and match them up with their corresponding Foundation Projects
func loadFOSSAProjects(db *gorm.DB, svc model.Service, token string) error {
	fossaClient := fossa.NewClient(token)
	s := NewSQLStore(db)

	pm, err := s.getProjectMaintainersMap()
	if err != nil {
		return fmt.Errorf("getProjectMaintainersMap failed: %w", err)
	}
	ftm, err := fossaClient.FetchTeamsMap()
	if err != nil {
		return fmt.Errorf("FetchTeamsMap failed: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for name, fossaTeam := range ftm {
			projectInfo, exists := pm[name]
			if !exists || projectInfo.Project.ID == 0 {
				continue
			}
			st := model.ServiceTeam{
				ProjectID: projectInfo.Project.ID,
				ServiceID: svc.ID,
				RemoteID:  fossaTeam.ID,
			}
			if err := tx.FirstOrCreate(&st).Error; err != nil {
				return fmt.Errorf("insert service team failed: %w", err)
			}
		}
		return nil
	})
}
