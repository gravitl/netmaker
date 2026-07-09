package logic

import (
	"context"
	"testing"
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
	_, err := AssignNodeExitNode(context.Background(), "", "node-1", "eg-1")
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
}
