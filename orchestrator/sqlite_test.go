//go:build sqlite

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestSqliteCENodeOrchestratorTestSuite(t *testing.T) {
	suite.Run(t, NewSqliteCENodeOrchestratorTestSuite())
}
