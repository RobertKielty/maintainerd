package db

import (
	"maintainerd/model"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateMaintainerDetails_RecomputesGeoOnLocationChange(t *testing.T) {
	gormDB := setupTestDB(t)
	store := NewSQLStore(gormDB)

	country := "DE"
	timezone := "Europe/Berlin"
	location := "Berlin, Germany"
	maintainer := model.Maintainer{
		Name:          "Alice Developer",
		Email:         "alice@example.com",
		GitHubAccount: "alice",
		Location:      &location,
		Country:       &country,
		Timezone:      &timezone,
	}
	require.NoError(t, gormDB.Create(&maintainer).Error)

	newLocation := "Paris, France"
	updated, err := store.UpdateMaintainerDetails(maintainer.ID, maintainer.Name, maintainer.Email, maintainer.GitHubAccount, "", &newLocation, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.Location)
	require.Equal(t, "Paris, France", *updated.Location)
	require.NotNil(t, updated.Country)
	require.Equal(t, "FR", *updated.Country)
	require.NotNil(t, updated.Timezone)
	require.Equal(t, "Europe/Paris", *updated.Timezone)
}

func TestUpdateMaintainerDetails_UnparseableUnchangedLocationKeepsGeo(t *testing.T) {
	gormDB := setupTestDB(t)
	store := NewSQLStore(gormDB)

	country := "PL"
	timezone := "Europe/Warsaw"
	location := "Kraków, PL"
	maintainer := model.Maintainer{
		Name:          "Bob Maintainer",
		Email:         "bob@example.com",
		GitHubAccount: "bob",
		Location:      &location,
		Country:       &country,
		Timezone:      &timezone,
	}
	require.NoError(t, gormDB.Create(&maintainer).Error)

	sameLocation := "Kraków, PL"
	updated, err := store.UpdateMaintainerDetails(maintainer.ID, maintainer.Name, maintainer.Email, maintainer.GitHubAccount, "", &sameLocation, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.Country)
	require.Equal(t, "PL", *updated.Country)
	require.NotNil(t, updated.Timezone)
	require.Equal(t, "Europe/Warsaw", *updated.Timezone)
}

func TestUpdateMaintainerDetails_ClearingLocationClearsGeo(t *testing.T) {
	gormDB := setupTestDB(t)
	store := NewSQLStore(gormDB)

	country := "DE"
	timezone := "Europe/Berlin"
	location := "Berlin, Germany"
	maintainer := model.Maintainer{
		Name:          "Carol Maintainer",
		Email:         "carol@example.com",
		GitHubAccount: "carol",
		Location:      &location,
		Country:       &country,
		Timezone:      &timezone,
	}
	require.NoError(t, gormDB.Create(&maintainer).Error)

	updated, err := store.UpdateMaintainerDetails(maintainer.ID, maintainer.Name, maintainer.Email, maintainer.GitHubAccount, "", nil, nil)
	require.NoError(t, err)
	require.Nil(t, updated.Location)
	require.Nil(t, updated.Country)
	require.Nil(t, updated.Timezone)
}

func TestUpdateMaintainerDetails_NameOnlyEditLeavesLocationUntouched(t *testing.T) {
	gormDB := setupTestDB(t)
	store := NewSQLStore(gormDB)

	country := "DE"
	timezone := "Europe/Berlin"
	location := "Berlin, Germany"
	maintainer := model.Maintainer{
		Name:          "Dave Maintainer",
		Email:         "dave@example.com",
		GitHubAccount: "dave",
		Location:      &location,
		Country:       &country,
		Timezone:      &timezone,
	}
	require.NoError(t, gormDB.Create(&maintainer).Error)

	updated, err := store.UpdateMaintainerDetails(maintainer.ID, "Dave M. Maintainer", maintainer.Email, maintainer.GitHubAccount, "", &location, nil)
	require.NoError(t, err)
	require.Equal(t, "Dave M. Maintainer", updated.Name)
	require.NotNil(t, updated.Location)
	require.Equal(t, "Berlin, Germany", *updated.Location)
	require.NotNil(t, updated.Country)
	require.Equal(t, "DE", *updated.Country)
	require.NotNil(t, updated.Timezone)
	require.Equal(t, "Europe/Berlin", *updated.Timezone)
}
