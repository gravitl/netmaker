package logic

import (
	"fmt"

	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
)

func CleanupGwsMigration() {
	acls := logic.ListAcls()
	for _, acl := range acls {
		upsert := false
		for i, srcI := range acl.Src {
			if srcI.ID == schema.NodeTagID && srcI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), schema.OldRemoteAccessTagName) {
				srcI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), schema.GwTagName)
				acl.Src[i] = srcI
				upsert = true
			}
		}
		for i, dstI := range acl.Dst {
			if dstI.ID == schema.NodeTagID && dstI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), schema.OldRemoteAccessTagName) {
				dstI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), schema.GwTagName)
				acl.Dst[i] = dstI
				upsert = true
			}
		}
		if upsert {
			logic.UpsertAcl(acl)
		}
	}
	nets, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	for _, netI := range nets {
		DeleteTag(schema.TagID(fmt.Sprintf("%s.%s", netI.Name, schema.OldRemoteAccessTagName)), true)
	}
}
