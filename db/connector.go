package db

import (
	"errors"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/gorm"
)

var ErrUnsupportedDB = errors.New("unsupported db type")

// connector helps connect to a database,
// along with any initializations required.
type connector interface {
	connect() (*gorm.DB, error)
}

// newConnector detects the database being
// used and returns the corresponding connector.
func newConnector() (connector, error) {
	switch servercfg.GetDB() {
	case "sqlite":
		return &sqliteConnector{}, nil
	case "postgres":
		return &postgresConnector{}, nil
	default:
		return nil, ErrUnsupportedDB
	}
}
