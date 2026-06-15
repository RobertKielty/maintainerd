package lfx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTokenUsesUserSearchEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		assert.Equal(t, "maintainer-d-token-check-no-such-user", r.URL.Query().Get("username"))
		assert.Equal(t, "1", r.URL.Query().Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "active-token",
	}

	require.NoError(t, client.CheckToken(context.Background()))
	assert.Equal(t, "/user-service/v2/users/search", gotPath)
	assert.Equal(t, "Bearer active-token", gotAuth)
}

func TestCheckTokenReturnsAccessError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expired", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "expired-token",
	}

	err := client.CheckToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
