//go:build postgres

package orchestrator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestPostgresCENodeOrchestratorTestSuite(t *testing.T) {
	if os.Getenv("CI") != "true" {
		suite.Run(t, NewPostgresCENodeOrchestratorTestSuite())
	}
}
