package model

import (
	"database/sql/driver"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type MaintainerStatus string

const (
	ActiveMaintainer   MaintainerStatus = "Active"
	EmeritusMaintainer MaintainerStatus = "Emeritus"
	RetiredMaintainer  MaintainerStatus = "Retired"
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

func (m *Maturity) Scan(value interface{ any }) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into Maturity", value)
	}
	*m = Maturity(v)
	return nil
}

type Maturity string

const (
	Sandbox    Maturity = "Sandbox"
	Incubating Maturity = "Incubating"
	Graduated  Maturity = "Graduated"
	Archived   Maturity = "Archived"
)

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

// ProjectInfo is an in-memory cache. TODO Review this
type ProjectInfo struct {
	Project     Project
	Maintainers []Maintainer
	Services    []Service
}
