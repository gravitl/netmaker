//go:build postgres

package orchestrator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestPostgresProNodeOrchestratorTestSuite(t *testing.T) {
	if os.Getenv("CI") != "true" {
		suite.Run(t, NewPostgresProNodeOrchestratorTestSuite())
	}
}
