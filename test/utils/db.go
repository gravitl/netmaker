package utils

import (
	"os"
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func InitSqlite(t *testing.T) {
	err := db.InitializeDB(schema.ListModels()...)
	require.Nil(t, err)

	err = database.InitializeDatabase()
	require.Nil(t, err)
}

func CleanupSqlite(t *testing.T) {
	db.CloseDB()

	err := os.RemoveAll("data")
	require.Nil(t, err)
}

func InitPostgres(t *testing.T) {
	_ = os.Setenv("DATABASE", "postgres")

	err := db.InitializeDB(schema.ListModels()...)
	require.Nil(t, err)

	err = database.InitializeDatabase()
	require.Nil(t, err)
}

func CleanupPostgres(_ *testing.T) {
	db.CloseDB()
}
