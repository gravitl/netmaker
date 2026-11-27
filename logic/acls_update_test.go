package logic

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestUpdateAclPersistsMetaData(t *testing.T) {
	ctx := scopedTestContext(t)
	insertPolicy := func(t *testing.T, meta string) models.Acl {
		t.Helper()
		acl := models.Acl{
			ID:        "metadata-update-" + uuid.NewString(),
			NetworkID: schema.NetworkID("metadata-update-net"),
			Name:      "before",
			MetaData:  meta,
			RuleType:  models.DevicePolicy,
			Enabled:   true,
		}
		if err := InsertAcl(ctx, acl); err != nil {
			t.Fatalf("insert acl: %v", err)
		}
		t.Cleanup(func() { _ = DeleteAcl(ctx, acl) })
		return acl
	}

	t.Run("sets the description", func(t *testing.T) {
		acl := insertPolicy(t, "before")
		updated := acl
		updated.MetaData = "after"
		if err := UpdateAcl(ctx, updated, acl); err != nil {
			t.Fatalf("update acl: %v", err)
		}
		got, err := GetAcl(ctx, acl.ID)
		if err != nil {
			t.Fatalf("get acl: %v", err)
		}
		if got.MetaData != "after" {
			t.Fatalf("expected meta_data %q, got %q", "after", got.MetaData)
		}
	})

	t.Run("empty keeps the stored description", func(t *testing.T) {
		acl := insertPolicy(t, "keep")
		updated := acl
		updated.MetaData = ""
		if err := UpdateAcl(ctx, updated, acl); err != nil {
			t.Fatalf("update acl: %v", err)
		}
		got, err := GetAcl(ctx, acl.ID)
		if err != nil {
			t.Fatalf("get acl: %v", err)
		}
		if got.MetaData != "keep" {
			t.Fatalf("expected meta_data %q to survive an empty request value, got %q", "keep", got.MetaData)
		}
	})
}
