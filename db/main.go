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
)

// MaintainerStatus is the lifecycle of a maintainer.
//
// The value is persisted as the literal string ", "Emeritus", …).
type MaintainerStatus string

const (
	ActiveMaintainer   MaintainerStatus = "Active"
	EmeritusMaintainer MaintainerStatus = "Emeritus"
	RetiredMaintainer  MaintainerStatus = "Retired"
	// Worksheet headings
	StatusHdr             string = "Status"
	ProjectHdr            string = "Project"
	MaintNameHdr          string = "Maintainer Name"
	CompanyNameHdr        string = "Company"
	EmailHdr              string = "Emails"
	GitHubHdr             string = "Github Name"
	ParentProjectHdr      string = "Parent Project"
	MaintainterFileRefHdr string = "OWNERS/MAINTAINERS"
	MailingListAddrHdr    string = "Mailing List Address"
	UpdateNoteHdr         string = "Update"
)

// IsValid" returns true id MaintainerStatus is known
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
	MailingList     *string `gorm:"size:254;default:MML_MISSING"`
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

func main() {

	spreadsheetId := os.Getenv("MD_WORKSHEET")
	if spreadsheetId == "" {
		log.Fatal("Environment variable MD_WORKSHEET is not set")
	}

	readRange := "Active!A1:J1639" // Adjust range as needed

	db, err := gorm.Open(sqlite.Open("demo.db"), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("failed opening database: %v", err)
	}

	// Auto‑migrate the schema. The order matters when foreign keys are
	// present, so we put parent tables first.
	if err := db.AutoMigrate(
		&Company{},
		&Project{},
		&Maintainer{},
		&MaintainerProject{},
	); err != nil {
		log.Fatalf("maintainerd: backend: auto‑migration failed: %v", err)
	}
	loadMaintainersAndProjects(db, spreadsheetId, readRange)

	log.Println("maintainerd: backend: database successfully seeded demo.db (SQLite)")
}

// Reads the readRange data from spreadsheetID inserts it into db
// The readRange from thw worksheet MUST include the header row
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

// readSheetRows returns every row as a map keyed by the header row,
// and carries forward the last non‐empty "Project" and "Status" values
// when those cells are blank or missing.
func readSheetRows(ctx context.Context, srv *sheets.Service, spreadsheetID, readRange string) ([]map[string]string, error) {
	resp, err := srv.Spreadsheets.Values.
		Get(spreadsheetID, readRange).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data: %w", err)
	}
	if len(resp.Values) == 0 {
		return nil, fmt.Errorf("sheet is empty")
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
