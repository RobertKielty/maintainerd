package db

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetProjectsUsingService(t *testing.T) {
	store := NewSQLStore(testDB)
	projects, err := store.GetProjectsUsingService(1)
	require.NoError(t, err)
	require.NotEmpty(t, projects)
}
