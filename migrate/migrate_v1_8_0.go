package migrate

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func migrateV1_8_0(ctx context.Context) error {
	return migrateAclAccessType(ctx)
}

// migrateAclAccessType sets network access on every acl saved before
// access_type existed. None of them were managed access policies.
func migrateAclAccessType(ctx context.Context) error {
	var aclRecords []schema.AclRecord
	if err := db.FromContext(ctx).Find(&aclRecords).Error; err != nil {
		return err
	}

	for _, record := range aclRecords {
		acl := record.Value.Data()
		if acl.AccessType != "" {
			continue
		}

		acl.AccessType = models.NetworkAccess

		record.Value = datatypes.NewJSONType(acl)
		err := db.FromContext(ctx).Model(&record).
			Update("value", record.Value).
			Error
		if err != nil {
			return err
		}
	}

	return nil
}
