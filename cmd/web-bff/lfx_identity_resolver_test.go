package main

import (
	"context"
	"testing"

	"maintainerd/lfx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUsernameOnlyClient simulates an LFX/PCC record with no GithubID field
// populated and no email, but whose LF Username (the openprofile.dev slug)
// happens to match the GitHub handle.
type fakeUsernameOnlyClient struct {
	usernameMatch lfx.User
}

func (f *fakeUsernameOnlyClient) SearchUsers(_ context.Context, query lfx.UserSearch) ([]lfx.User, error) {
	if query.Username != "" {
		return []lfx.User{f.usernameMatch}, nil
	}
	return nil, nil
}

func (f *fakeUsernameOnlyClient) GetUserIdentities(context.Context, string) ([]lfx.Identity, error) {
	return nil, nil
}

func TestLFXIdentityResolverFallsBackToUsernameSearch(t *testing.T) {
	t.Parallel()

	resolver := lfxIdentityResolver{
		client: &fakeUsernameOnlyClient{
			usernameMatch: lfx.User{
				ID:        "sfid-fixture-4",
				Username:  "fixture-handle",
				FirstName: "Fixture",
				LastName:  "Person",
				Type:      "contact",
			},
		},
	}

	result, err := resolver.ResolveMaintainerIdentity(context.Background(), "fixture-handle", "")
	require.NoError(t, err)

	assert.Equal(t, "weak", result.Confidence, "a username-only match is a coincidental string match, not a verified linkage, so it must not inherit strong")
	assert.Equal(t, "single LFX user match by username only", result.Reason)
	assert.Equal(t, "Fixture Person", result.Name, "the matched record's name should be lifted even though the match came from the username fallback")
	assert.Equal(t, "fixture-handle", result.LFID)
}

func TestLFXIdentityResolverGitHubIDMatchStaysStrong(t *testing.T) {
	t.Parallel()

	resolver := lfxIdentityResolver{
		client: &fakeUsernameOnlyClientWithGitHubID{
			githubIDMatch: lfx.User{
				ID:        "sfid-fixture-5",
				Username:  "fixture-handle",
				FirstName: "Other",
				LastName:  "Fixture",
				Type:      "contact",
			},
		},
	}

	result, err := resolver.ResolveMaintainerIdentity(context.Background(), "fixture-handle", "")
	require.NoError(t, err)

	assert.Equal(t, "strong", result.Confidence, "a real GithubID match must keep its existing strong confidence, unaffected by the new username fallback")
	assert.Equal(t, "single LFX user match", result.Reason)
}

// fakeUsernameOnlyClientWithGitHubID simulates a record actually found via
// the GitHubID search, so the username fallback path must never fire.
type fakeUsernameOnlyClientWithGitHubID struct {
	githubIDMatch lfx.User
}

func (f *fakeUsernameOnlyClientWithGitHubID) SearchUsers(_ context.Context, query lfx.UserSearch) ([]lfx.User, error) {
	if query.GitHubID != "" {
		return []lfx.User{f.githubIDMatch}, nil
	}
	return nil, nil
}

func (f *fakeUsernameOnlyClientWithGitHubID) GetUserIdentities(context.Context, string) ([]lfx.Identity, error) {
	return nil, nil
}
