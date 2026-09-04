package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestListNodeExitNodes_Validation(t *testing.T) {
	_, err := ListNodeExitNodes(context.Background(), "", "node-1")
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
	_, err = ListNodeExitNodes(context.Background(), "net-1", "")
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAssignNodeExitNode_Validation(t *testing.T) {
	_, err := AssignNodeExitNode(context.Background(), "", "node-1", "eg-1", false)
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestValidateInternetEgressSelection_ExitNodeCannotUseAnotherExitNode(t *testing.T) {
	node := &models.Node{}
	node.ID = uuid.New()
	node.Network = "net-1"

	err := validateInternetEgressSelection(node, &schema.Node{ID: node.ID.String()}, uuid.NewString(), true)
	if err == nil || err.Error() != "exit node cannot use another exit node" {
		t.Fatalf("expected exit-node validation error, got %v", err)
	}
}
