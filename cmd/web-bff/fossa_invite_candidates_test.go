package main

import (
	"testing"

	"maintainerd/model"
)

// TestBuildFossaInviteCandidates_MatchesGitHubEmail guards against the
// reconciliation gap where an invite sent to a maintainer's GitHubEmail
// (see preferredMaintainerServiceEmail) could never be recognized as
// "already invited" when eligibility was only checked against Email.
func TestBuildFossaInviteCandidates_MatchesGitHubEmail(t *testing.T) {
	maintainer := model.Maintainer{
		Name:          "Ada Lovelace",
		Email:         "ada@example.com",
		GitHubEmail:   "ada@users.noreply.github.com",
		GitHubAccount: "adal",
	}
	maintainer.ID = 1

	projectStatuses := map[uint]model.MaintainerStatus{1: model.ActiveMaintainer}

	t.Run("pending invite recorded under GitHubEmail excludes the maintainer", func(t *testing.T) {
		pendingInviteEmails := map[string]struct{}{"ada@users.noreply.github.com": {}}
		got := buildFossaInviteCandidates([]model.Maintainer{maintainer}, projectStatuses, nil, pendingInviteEmails)
		if len(got) != 0 {
			t.Fatalf("expected no candidates, got %+v", got)
		}
	})

	t.Run("FOSSA team membership recorded under GitHubEmail excludes the maintainer", func(t *testing.T) {
		got := buildFossaInviteCandidates([]model.Maintainer{maintainer}, projectStatuses, []string{"ada@users.noreply.github.com"}, map[string]struct{}{})
		if len(got) != 0 {
			t.Fatalf("expected no candidates, got %+v", got)
		}
	})

	t.Run("no match leaves the maintainer eligible", func(t *testing.T) {
		got := buildFossaInviteCandidates([]model.Maintainer{maintainer}, projectStatuses, nil, map[string]struct{}{})
		if len(got) != 1 {
			t.Fatalf("expected 1 candidate, got %+v", got)
		}
	})
}
