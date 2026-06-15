package dotproject

import (
	"context"
	"strings"
	"testing"

	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fakeAutoAddStore struct {
	maintainers map[string]model.Maintainer
	linked      map[uint]map[uint]bool
	observed    []model.MaintainerIdentityObservation
	audits      []model.AuditLog
	nextID      uint
}

func newFakeAutoAddStore() *fakeAutoAddStore {
	return &fakeAutoAddStore{
		maintainers: make(map[string]model.Maintainer),
		linked:      make(map[uint]map[uint]bool),
		nextID:      100,
	}
}

func (f *fakeAutoAddStore) GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error) {
	result := make(map[string]model.Maintainer, len(f.maintainers))
	for key, maintainer := range f.maintainers {
		result[key] = maintainer
	}
	return result, nil
}

func (f *fakeAutoAddStore) GetMaintainersByProject(projectID uint) ([]model.Maintainer, error) {
	var result []model.Maintainer
	for _, maintainer := range f.maintainers {
		if f.linked[maintainer.ID][projectID] {
			result = append(result, maintainer)
		}
	}
	return result, nil
}

func (f *fakeAutoAddStore) UpsertMaintainerWithIdentity(projectID uint, name, email, githubHandle, company, lfxUserID string) (*model.Maintainer, bool, bool, error) {
	key := NormalizeGitHubHandle(githubHandle)
	maintainer, ok := f.maintainers[key]
	created := false
	if !ok {
		f.nextID++
		maintainer = model.Maintainer{
			Model:            gorm.Model{ID: f.nextID},
			Name:             strings.TrimSpace(name),
			Email:            strings.TrimSpace(email),
			GitHubAccount:    strings.TrimSpace(githubHandle),
			LFXUserID:        strings.TrimSpace(lfxUserID),
			MaintainerStatus: model.ActiveMaintainer,
			Company:          model.Company{Name: company},
		}
		f.maintainers[key] = maintainer
		created = true
	}
	if f.linked[maintainer.ID] == nil {
		f.linked[maintainer.ID] = make(map[uint]bool)
	}
	linked := !f.linked[maintainer.ID][projectID]
	f.linked[maintainer.ID][projectID] = true
	return &maintainer, created, linked, nil
}

func (f *fakeAutoAddStore) UpsertMaintainerIdentityObservation(observation *model.MaintainerIdentityObservation) (*model.MaintainerIdentityObservation, error) {
	f.observed = append(f.observed, *observation)
	return observation, nil
}

func (f *fakeAutoAddStore) GetLatestMaintainerIdentityObservation(source string, maintainerID uint) (*model.MaintainerIdentityObservation, error) {
	for i := len(f.observed) - 1; i >= 0; i-- {
		observation := f.observed[i]
		if observation.Source == source && observation.MaintainerID != nil && *observation.MaintainerID == maintainerID {
			copy := observation
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeAutoAddStore) GetLatestMaintainerIdentityObservationByRef(source string, projectID uint, sourceRef string) (*model.MaintainerIdentityObservation, error) {
	for i := len(f.observed) - 1; i >= 0; i-- {
		observation := f.observed[i]
		if observation.Source == source && observation.ProjectID != nil && *observation.ProjectID == projectID && observation.SourceRef == sourceRef {
			copy := observation
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeAutoAddStore) LogAuditEvent(_ *zap.SugaredLogger, event model.AuditLog) error {
	f.audits = append(f.audits, event)
	return nil
}

type fakeLFXResolver struct {
	result LFXIdentityResult
	err    error
	calls  int
}

func (f *fakeLFXResolver) ResolveMaintainerIdentity(_ context.Context, _, _ string) (LFXIdentityResult, error) {
	f.calls++
	return f.result, f.err
}

func TestAutoMaintainerAdderDryRunWritesObservationsButNoMaintainers(t *testing.T) {
	t.Parallel()

	index, err := ParseFoundationMaintainersCSV(strings.NewReader(`,Project,Maintainer Name,Company,Github Name
Graduated,Kubernetes,Alice Example,Acme,AliceExample
`))
	require.NoError(t, err)
	index.SourceURL = "https://github.com/cncf/foundation/blob/abc/project-maintainers.csv"
	index.CommitSHA = "abc"

	store := newFakeAutoAddStore()
	adder := &AutoMaintainerAdder{
		Store:              store,
		Foundation:         index,
		CheckFoundationCSV: true,
		AutoAddMaintainers: false,
	}

	summary, err := adder.ProcessProject(context.Background(), model.Project{Model: gorm.Model{ID: 1}, Name: "Kubernetes"}, &DiscoveryResult{
		MaintainersFile: FileDiscovery{Exists: true, Body: `maintainers:
  - teams:
      - name: project-maintainers
        members:
          - AliceExample
          - missing-handle
`},
	})
	require.NoError(t, err)

	assert.Equal(t, 2, summary.Candidates)
	assert.Equal(t, 1, summary.WouldCreateMaintainers)
	assert.Equal(t, 1, summary.SkippedFoundationMissing)
	assert.Len(t, summary.WouldCreate, 1)
	assert.Equal(t, "Kubernetes", summary.WouldCreate[0].Project)
	assert.Equal(t, "AliceExample", summary.WouldCreate[0].GitHub)
	assert.Empty(t, store.maintainers)
	require.Len(t, store.observed, 2)
	statuses := make(map[string]bool)
	for _, observation := range store.observed {
		assert.Equal(t, FoundationCSVSource, observation.Source)
		statuses[observation.MatchStatus] = true
	}
	assert.True(t, statuses["matched"])
	assert.True(t, statuses["unmatched"])
}

func TestAutoMaintainerAdderSkipsInvalidMaintainersFile(t *testing.T) {
	t.Parallel()

	store := newFakeAutoAddStore()
	adder := &AutoMaintainerAdder{
		Store:              store,
		CheckFoundationCSV: false,
		AutoAddMaintainers: false,
	}

	summary, err := adder.ProcessProject(context.Background(), model.Project{Model: gorm.Model{ID: 1}, Name: "Kubernetes"}, &DiscoveryResult{
		MaintainersFile: FileDiscovery{Exists: true, Body: `maintainers: []`},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, summary.SkippedInvalidMaintainers)
	assert.Equal(t, 0, summary.Candidates)
	assert.Empty(t, store.maintainers)
	assert.Empty(t, store.observed)
	assert.Empty(t, store.audits)
}

func TestAutoMaintainerAdderDryRunIncludesLFXIdentityInWouldCreateSummary(t *testing.T) {
	t.Parallel()

	index, err := ParseFoundationMaintainersCSV(strings.NewReader(`,Project,Maintainer Name,Company,Github Name
Graduated,Kubernetes,Alice Example,Acme,AliceExample
`))
	require.NoError(t, err)

	store := newFakeAutoAddStore()
	adder := &AutoMaintainerAdder{
		Store:              store,
		Foundation:         index,
		LFX:                &fakeLFXResolver{result: LFXIdentityResult{UserID: "003-test", LFID: "alice-lfid", Name: "Alice LFX", Email: "alice.lfx@example.com", Company: "LFX Co", Confidence: "exact"}},
		CheckFoundationCSV: true,
		AutoAddMaintainers: false,
	}

	summary, err := adder.ProcessProject(context.Background(), model.Project{Model: gorm.Model{ID: 1}, Name: "Kubernetes"}, &DiscoveryResult{
		MaintainersFile: FileDiscovery{Exists: true, Body: `maintainers:
  - teams:
      - name: project-maintainers
        members: [AliceExample]
`},
	})
	require.NoError(t, err)

	require.Len(t, summary.WouldCreate, 1)
	assert.Equal(t, "alice-lfid", summary.WouldCreate[0].LFXID)
	assert.Equal(t, "Alice LFX", summary.WouldCreate[0].Name)
	assert.Equal(t, "LFX Co", summary.WouldCreate[0].Company)
	assert.Equal(t, "alice.lfx@example.com", summary.WouldCreate[0].Email)
}

func TestAutoMaintainerAdderDryRunUsesSavedLFXObservationForWouldCreateSummary(t *testing.T) {
	t.Parallel()

	index, err := ParseFoundationMaintainersCSV(strings.NewReader(`,Project,Maintainer Name,Company,Github Name
Graduated,Kubernetes,Alice Example,Acme,AliceExample
`))
	require.NoError(t, err)

	store := newFakeAutoAddStore()
	projectID := uint(1)
	store.observed = append(store.observed, model.MaintainerIdentityObservation{
		ProjectID:    &projectID,
		Source:       "lfx",
		SourceRef:    "github:aliceexample",
		SourceUserID: "003-test",
		LFID:         "alice-lfid",
		Name:         "Alice LFX",
		CompanyName:  "LFX Co",
		Email:        "alice.lfx@example.com",
		MatchStatus:  "matched",
	})
	adder := &AutoMaintainerAdder{
		Store:              store,
		Foundation:         index,
		CheckFoundationCSV: true,
		AutoAddMaintainers: false,
	}

	summary, err := adder.ProcessProject(context.Background(), model.Project{Model: gorm.Model{ID: projectID}, Name: "Kubernetes"}, &DiscoveryResult{
		MaintainersFile: FileDiscovery{Exists: true, Body: `maintainers:
  - teams:
      - name: project-maintainers
        members: [AliceExample]
`},
	})
	require.NoError(t, err)

	require.Len(t, summary.WouldCreate, 1)
	assert.Equal(t, "alice-lfid", summary.WouldCreate[0].LFXID)
	assert.Equal(t, "Alice LFX", summary.WouldCreate[0].Name)
	assert.Equal(t, "LFX Co", summary.WouldCreate[0].Company)
	assert.Equal(t, "alice.lfx@example.com", summary.WouldCreate[0].Email)
}

func TestAutoMaintainerAdderWriteModeLinksExistingMaintainer(t *testing.T) {
	t.Parallel()

	index, err := ParseFoundationMaintainersCSV(strings.NewReader(`,Project,Maintainer Name,Company,Github Name
Graduated,Kubernetes,Alice Example,Acme,AliceExample
`))
	require.NoError(t, err)

	store := newFakeAutoAddStore()
	existing := model.Maintainer{
		Model:            gorm.Model{ID: 7},
		Name:             "Alice Existing",
		Email:            "alice.lfx@example.com",
		GitHubAccount:    "aliceexample",
		MaintainerStatus: model.ActiveMaintainer,
	}
	store.maintainers["aliceexample"] = existing
	adder := &AutoMaintainerAdder{
		Store:              store,
		Foundation:         index,
		CheckFoundationCSV: true,
		AutoAddMaintainers: true,
	}

	summary, err := adder.ProcessProject(context.Background(), model.Project{Model: gorm.Model{ID: 1}, Name: "Kubernetes"}, &DiscoveryResult{
		MaintainersFile: FileDiscovery{Exists: true, Body: `maintainers:
  - teams:
      - name: project-maintainers
        members: [AliceExample]
`},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, summary.LinkedMaintainers)
	assert.True(t, store.linked[existing.ID][1])
	require.Len(t, store.audits, 1)
	assert.Equal(t, "ADD_DOT_PROJECT_MAINTAINER", store.audits[0].Action)
	assert.Contains(t, store.audits[0].Message, "aliceexample was added")
}
