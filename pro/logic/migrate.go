package logic

import (
	"fmt"

	"context"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func CleanupGwsMigration(ctx context.Context) {
	acls := logic.ListAcls(ctx)
	for _, acl := range acls {
		upsert := false
		for i, srcI := range acl.Src {
			if srcI.ID == models.NodeTagID && srcI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				srcI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Src[i] = srcI
				upsert = true
			}
		}
		for i, dstI := range acl.Dst {
			if dstI.ID == models.NodeTagID && dstI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				dstI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Dst[i] = dstI
				upsert = true
			}
		}
		if upsert {
			logic.UpsertAcl(ctx, acl)
		}
	}
	nets, _ := (&schema.Network{}).ListAll(ctx)
	for _, netI := range nets {
		DeleteTag(ctx, models.TagID(fmt.Sprintf("%s.%s", netI.Name, models.OldRemoteAccessTagName)), true)
	}
}
