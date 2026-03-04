package onboarding

import "testing"

func TestGetProjectNameFromProjectTitle(t *testing.T) {
	t.Run("extracts project name from standard onboarding title", func(t *testing.T) {
		name, err := GetProjectNameFromProjectTitle("[PROJECT ONBOARDING] Example Project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "Example Project" {
			t.Fatalf("expected %q, got %q", "Example Project", name)
		}
	})

	t.Run("rejects missing prefix", func(t *testing.T) {
		_, err := GetProjectNameFromProjectTitle("PROJECT ONBOARDING Example Project")
		if err == nil {
			t.Fatalf("expected error for missing prefix")
		}
	})
}
