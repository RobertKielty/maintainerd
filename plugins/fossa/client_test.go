package fossa_test

import (
	"maintainerd/plugins/fossa"
	"os"
	"testing"
)

func TestFetchUserInvitations_Live(t *testing.T) {
	apiKey := os.Getenv("FOSSA_API_TOKEN")
	if apiKey == "" {
		t.Skip("FOSSA_API_TOKEN not set; skipping live API test")
	}

	client := fossa.NewClient(apiKey)

	body, err := client.FetchUserInvitations()
	if err != nil {
		t.Fatalf("FetchUserInvitations returned error: %v", err)
	}

	t.Logf("FetchUserInvitations response: %s", body)
}
