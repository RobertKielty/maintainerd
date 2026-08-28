package lfx

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"maintainerd/dotproject"
	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeObservationStore struct {
	maintainers map[string]model.Maintainer
	captured    *model.MaintainerIdentityObservation
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

func (f fakeObservationStore) UpsertMaintainerIdentityObservation(observation *model.MaintainerIdentityObservation) (*model.MaintainerIdentityObservation, error) {
	if f.captured != nil {
		*f.captured = *observation
	}
	return observation, nil
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

// fakeSingleUserSearcher returns a fixed single-user match, with Raw bytes
// captured exactly as the real Client.SearchUsers/GetUserIdentities do.
type fakeSingleUserSearcher struct {
	user       User
	identities []Identity
}

func (f *fakeSingleUserSearcher) SearchUsers(context.Context, UserSearch) ([]User, error) {
	return []User{f.user}, nil
}

func (f *fakeSingleUserSearcher) GetUserIdentities(context.Context, string) ([]Identity, error) {
	return f.identities, nil
}

func TestWriteObservationPreservesRawBytesBeyondTypedFields(t *testing.T) {
	t.Parallel()

	// "Nickname" is not a field on User. It stands in for any real LFX
	// response field we haven't modeled yet - the point of this test is
	// that such fields survive into RawPayload instead of being dropped.
	userRaw := []byte(`{"ID":"sfid-synthetic-001","FirstName":"Test","LastName":"Fixture","Name":"Test Fixture","Email":"fixture@example.com","Username":"test-fixture-handle","Account":{"Name":"Example Org"},"Nickname":"unmodelled-field-value"}`)
	var user User
	require.NoError(t, unmarshalStrict(userRaw, &user))
	user.Raw = userRaw

	identityRaw := []byte(`{"ID":"identity-synthetic-001","UserSFID":"sfid-synthetic-001","Username":"test-fixture-handle","Source":"github","IsVerified":true,"Badge":"unmodelled-identity-field"}`)
	var identity Identity
	require.NoError(t, unmarshalStrict(identityRaw, &identity))
	identity.Raw = identityRaw

	var captured model.MaintainerIdentityObservation
	enricher := &Enricher{
		Store: fakeObservationStore{maintainers: map[string]model.Maintainer{}, captured: &captured},
		Client: &fakeSingleUserSearcher{
			user:       user,
			identities: []Identity{identity},
		},
	}

	var summary dotproject.EnrichmentSummary
	err := enricher.enrichCandidate(context.Background(), nil, candidate{
		GitHubUser: "test-fixture-handle",
		SourceRef:  "github:test-fixture-handle",
	}, time.Now().UTC(), &summary)
	require.NoError(t, err)

	assert.Contains(t, captured.RawPayload, "unmodelled-field-value", "raw bytes beyond the typed struct must survive into RawPayload")
	assert.Contains(t, captured.RawPayload, "unmodelled-identity-field")

	// Typed extraction must still work off the preserved raw bytes.
	assert.Equal(t, "sfid-synthetic-001", captured.SourceUserID)
	assert.Equal(t, "Test Fixture", captured.Name)
	assert.Equal(t, "fixture@example.com", captured.Email)
	assert.Equal(t, "test-fixture-handle", captured.LFID)
	assert.Equal(t, "Example Org", captured.CompanyName)
	assert.Equal(t, "exact", captured.Confidence)
}

// unmarshalStrict is a small helper to decode fixture JSON into a typed
// struct while keeping the test's raw bytes and the decoded struct
// authored from the same literal, avoiding drift between them.
func unmarshalStrict(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
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
