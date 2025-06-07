package db

import (
	"errors"
	"os"

	"github.com/gravitl/netmaker/config"
	"gorm.io/gorm"
)

var ErrUnsupportedDB = errors.New("unsupported db type")

// connector helps connect to a database,
// along with any initializations required.
type connector interface {
	connect() (*gorm.DB, error)
}

// GetDB - gets the database type
func GetDB() string {
	database := "sqlite"
	if os.Getenv("DATABASE") != "" {
		database = os.Getenv("DATABASE")
	} else if config.Config.Server.Database != "" {
		database = config.Config.Server.Database
	}
	return database
}

// newConnector detects the database being
// used and returns the corresponding connector.
func newConnector() (connector, error) {
	switch GetDB() {
	case "sqlite":
		return &sqliteConnector{}, nil
	case "postgres":
		return &postgresConnector{}, nil
	default:
		return nil, ErrUnsupportedDB
	}
}
