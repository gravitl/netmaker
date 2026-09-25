package migrate

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// TODO: wire this up. It needs a migrationJobV180 ("migration-v1.8.0") in
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
