//go:build sqlite

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestSqliteProNodeOrchestratorTestSuite(t *testing.T) {
	suite.Run(t, NewSqliteProNodeOrchestratorTestSuite())
}
