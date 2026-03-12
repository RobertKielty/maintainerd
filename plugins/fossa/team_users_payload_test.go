package fossa

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveTeamAdminRoleID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/roles" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":1,"scope":"organization","name":"Admin"},
			{"id":4,"scope":"team","name":"Team Admin"},
			{"id":5,"scope":"team","name":"Team Editor"}
		]`))
	}))
	defer server.Close()

	client := &Client{
		APIKey:  "test-token",
		APIBase: server.URL,
	}

	roleID, err := client.ResolveTeamAdminRoleID()
	if err != nil {
		t.Fatalf("ResolveTeamAdminRoleID returned error: %v", err)
	}
	if roleID != 4 {
		t.Fatalf("roleID = %d, want %d", roleID, 4)
	}
}

func TestAddUserToTeamByEmailWithResponse_UsesCompatibilityPayloadShape(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":94780,"email":"member@example.com"}]`))
		case "/teams/102209/users":
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")

			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":102209,"users":[{"userId":94780,"roleId":3}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{
		APIKey:  "test-token",
		APIBase: server.URL,
	}

	resp, err := client.AddUserToTeamByEmailWithResponse(102209, "member@example.com", 3)
	if err != nil {
		t.Fatalf("AddUserToTeamByEmailWithResponse returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("AddUserToTeamByEmailWithResponse returned nil response")
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPut)
	}
	if gotPath != "/teams/102209/users" {
		t.Fatalf("path = %q, want %q", gotPath, "/teams/102209/users")
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotBody["action"] != "add" {
		t.Fatalf("action = %#v, want %q", gotBody["action"], "add")
	}

	users, ok := gotBody["users"].([]any)
	if !ok || len(users) != 1 {
		t.Fatalf("users = %#v, want single-element array", gotBody["users"])
	}

	user, ok := users[0].(map[string]any)
	if !ok {
		t.Fatalf("users[0] = %#v, want object", users[0])
	}
	if user["id"] != float64(94780) {
		t.Fatalf("users[0].id = %#v, want %v", user["id"], 94780)
	}
	if user["userId"] != float64(94780) {
		t.Fatalf("users[0].userId = %#v, want %v", user["userId"], 94780)
	}
	if user["roleId"] != float64(3) {
		t.Fatalf("users[0].roleId = %#v, want %v", user["roleId"], 3)
	}
}
