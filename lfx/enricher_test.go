package lfx

import (
	"context"
	"testing"

	"maintainerd/dotproject"
	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeObservationStore struct {
	maintainers map[string]model.Maintainer
}

func (f fakeObservationStore) GetMaintainerMapByGitHubAccount() (map[string]model.Maintainer, error) {
	result := make(map[string]model.Maintainer, len(f.maintainers))
	for key, maintainer := range f.maintainers {
		result[key] = maintainer
	}
	return result, nil
}

func (f fakeObservationStore) ListMaintainersWithoutIdentityObservation(string) ([]model.Maintainer, error) {
	return nil, nil
}

func (f fakeObservationStore) ListMaintainersActiveOnAnyProject(maintainerIDs []uint) (map[uint]bool, error) {
	active := make(map[uint]bool, len(maintainerIDs))
	for _, id := range maintainerIDs {
		active[id] = true
	}
	return active, nil
}

func (f fakeObservationStore) UpsertMaintainerIdentityObservation(*model.MaintainerIdentityObservation) (*model.MaintainerIdentityObservation, error) {
	return nil, nil
}

type fakeUserSearcher struct {
	calls int
}

func (f *fakeUserSearcher) SearchUsers(context.Context, UserSearch) ([]User, error) {
	f.calls++
	return nil, nil
}

func (f *fakeUserSearcher) GetUserIdentities(context.Context, string) ([]Identity, error) {
	f.calls++
	return nil, nil
}

func TestEnricherSkipsInvalidProjectMaintainersFile(t *testing.T) {
	t.Parallel()

	client := &fakeUserSearcher{}
	enricher := &Enricher{
		Store: fakeObservationStore{
			maintainers: map[string]model.Maintainer{},
		},
		Client: client,
	}

	summary, err := enricher.EnrichProject(context.Background(), model.Project{Model: gorm.Model{ID: 1}, Name: "Kubernetes"}, &dotproject.DiscoveryResult{
		MaintainersFile: dotproject.FileDiscovery{
			Exists: true,
			Body:   "maintainers: []",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, dotproject.EnrichmentSummary{}, summary)
	assert.Equal(t, 0, client.calls)
}
