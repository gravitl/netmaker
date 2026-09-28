package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func migrateV1_8_0(ctx context.Context) error {
	for _, migrate := range []migrationFunc{
		migrateExtClients,
		migrateAcls,
		migrateTags,
		migrateDNSEntries,
		migrateUserGroups,
		migrateUserRoles,
	} {
		if err := migrate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// networkIDs resolves network names to network ids within tenants, caching
// the results. An empty id is returned for networks that don't exist.
type networkIDs map[string]map[string]string

func (n networkIDs) get(ctx context.Context, tenantID, name string) (string, error) {
	if n[tenantID] == nil {
		n[tenantID] = make(map[string]string)
	}
	if id, ok := n[tenantID][name]; ok {
		return id, nil
	}

	var network schema.Network
	err := db.FromContext(ctx).Model(&schema.Network{}).
		Where("tenant_id = ? AND name = ?", tenantID, name).
		First(&network).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	n[tenantID][name] = network.ID
	return network.ID, nil
}

// unscopedKey returns the key without the tenant prefix it was stored with
// before v1.8.0.
func unscopedKey(tenantID, key string) string {
	return strings.TrimPrefix(key, tenantID+"::")
}

// idForSlug returns the slug as the id if it's a uuid, so that the ids of
// custom resources are kept, else a new uuid.
func idForSlug(slug string) string {
	if _, err := uuid.Parse(slug); err == nil && len(slug) == 36 {
		return slug
	}
	return uuid.NewString()
}

func migrateExtClients(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(&types.ExtClientRecord{}) {
		return nil
	}

	var records []types.ExtClientRecord
	if err := db.FromContext(ctx).Find(&records).Error; err != nil {
		return err
	}

	networks := make(networkIDs)
	for _, record := range records {
		extclient := record.Value.Data()
		if extclient.ClientID == "" {
			logger.Log(0, fmt.Sprintf("skipping migration of extclient record %s: no client id", record.Key))
			continue
		}

		networkID, err := networks.get(ctx, record.TenantID, extclient.Network)
		if err != nil {
			return err
		}
		if networkID == "" {
			logger.Log(0, fmt.Sprintf(
				"skipping migration of extclient %s: network %s (tenant %s) not found",
				extclient.ClientID, extclient.Network, record.TenantID,
			))
			continue
		}

		_extclient := &schema.Extclient{
			ID:                          uuid.NewString(),
			TenantID:                    record.TenantID,
			NetworkID:                   networkID,
			Name:                        extclient.ClientID,
			PrivateKey:                  extclient.PrivateKey,
			PublicKey:                   extclient.PublicKey,
			DNS:                         extclient.DNS,
			Address:                     extclient.Address,
			Address6:                    extclient.Address6,
			IngressGatewayID:            extclient.IngressGatewayID,
			IngressGatewayEndpoint:      extclient.IngressGatewayEndpoint,
			AllowedIPs:                  extclient.AllowedIPs,
			ExtraAllowedIPs:             extclient.ExtraAllowedIPs,
			PostUp:                      extclient.PostUp,
			PostDown:                    extclient.PostDown,
			SelectedInternetEgressID:    extclient.SelectedInternetEgressID,
			Enabled:                     extclient.Enabled,
			OwnerID:                     extclient.OwnerID,
			Status:                      extclient.Status,
			PostureCheckSeverity:        extclient.PostureCheckVolationSeverityLevel,
			PostureCheckLastEvaluatedAt: extclient.LastEvaluatedAt,
			RemoteAccessClientID:        extclient.RemoteAccessClientID,
			OS:                          extclient.OS,
			OSFamily:                    extclient.OSFamily,
			OSVersion:                   extclient.OSVersion,
			KernelVersion:               extclient.KernelVersion,
			ClientVersion:               extclient.ClientVersion,
			DeviceID:                    extclient.DeviceID,
			DeviceName:                  extclient.DeviceName,
			PublicEndpoint:              extclient.PublicEndpoint,
			Country:                     extclient.Country,
			Location:                    extclient.Location,
			JITExpiresAt:                extclient.JITExpiresAt,
			DeniedACLs:                  datatypes.NewJSONType(extclient.DeniedACLs),
			Tags:                        datatypes.NewJSONType(extclient.Tags),
		}

		if extclient.LastModified != 0 {
			lastModified := time.Unix(extclient.LastModified, 0).UTC()
			_extclient.CreatedAt = lastModified
			_extclient.UpdatedAt = lastModified
		}

		var violations []schema.PostureCheckViolation
		if len(extclient.PostureChecksViolations) > 0 {
			_extclient.PostureCheckLastEvaluationCycleID = uuid.NewString()
			for _, violation := range extclient.PostureChecksViolations {
				violations = append(violations, schema.PostureCheckViolation{
					EvaluationCycleID: _extclient.PostureCheckLastEvaluationCycleID,
					TenantID:          record.TenantID,
					CheckID:           violation.CheckID,
					SubjectID:         _extclient.ID,
					SubjectType:       schema.PostureCheckSubjectTypeExtclient,
					Name:              violation.Name,
					Attribute:         violation.Attribute,
					Message:           violation.Message,
					Severity:          violation.Severity,
					EvaluatedAt:       extclient.LastEvaluatedAt,
				})
			}
		}

		if err := _extclient.Create(ctx); err != nil {
			return fmt.Errorf("failed to migrate extclient %s of network %s (tenant %s): %w",
				extclient.ClientID, extclient.Network, record.TenantID, err)
		}
		if len(violations) > 0 {
			if err := db.FromContext(ctx).Create(&violations).Error; err != nil {
				return fmt.Errorf("failed to migrate posture check violations of extclient %s of network %s (tenant %s): %w",
					extclient.ClientID, extclient.Network, record.TenantID, err)
			}
		}
	}

	return nil
}

// migrateAcls copies the acls from the key-value acls table to acls_v1,
// identified by their slugs. The old table is left as is.
func migrateAcls(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(&types.AclRecord{}) {
		return nil
	}

	var records []types.AclRecord
	if err := db.FromContext(ctx).Find(&records).Error; err != nil {
		return err
	}

	for _, record := range records {
		slug := unscopedKey(record.TenantID, record.Key)
		networkID := record.NetworkID
		if networkID == "" {
			networkID = record.Value.Data().NetworkID.String()
		}
		acl := &schema.AclRecord{
			ID:        idForSlug(slug),
			TenantID:  record.TenantID,
			Slug:      slug,
			NetworkID: networkID,
			Value:     record.Value,
		}
		if err := db.FromContext(ctx).Create(acl).Error; err != nil {
			return fmt.Errorf("failed to migrate acl %s (tenant %s): %w", slug, record.TenantID, err)
		}
	}

	return nil
}

// migrateTags copies the tags from the key-value tags table to tags_v1,
// resolving their networks by name within their tenants. Tags of networks
// that no longer exist are skipped. The old table is left as is.
func migrateTags(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(&types.TagRecord{}) {
		return nil
	}

	var records []types.TagRecord
	if err := db.FromContext(ctx).Find(&records).Error; err != nil {
		return err
	}

	networks := make(networkIDs)
	for _, record := range records {
		tag := record.Value.Data()
		networkID, err := networks.get(ctx, record.TenantID, tag.Network.String())
		if err != nil {
			return err
		}
		if networkID == "" {
			logger.Log(0, fmt.Sprintf(
				"skipping migration of tag %s: network %s (tenant %s) not found",
				tag.ID, tag.Network, record.TenantID,
			))
			continue
		}

		// tags are referred to by <network>.<tag name>, which is how they're
		// looked up after the migration.
		if expected := schema.TagID(fmt.Sprintf("%s.%s", tag.Network, tag.TagName)); tag.ID != expected {
			logger.Log(0, fmt.Sprintf(
				"migrating tag %s (tenant %s) as %s, its references by id need to be updated",
				tag.ID, record.TenantID, expected,
			))
		}

		_tag := &schema.Tag{
			ID:        uuid.NewString(),
			TenantID:  record.TenantID,
			NetworkID: networkID,
			Name:      tag.TagName,
			ColorCode: tag.ColorCode,
			CreatedBy: tag.CreatedBy,
			CreatedAt: tag.CreatedAt,
		}
		if err := _tag.Create(ctx); err != nil {
			return fmt.Errorf("failed to migrate tag %s (tenant %s): %w", tag.ID, record.TenantID, err)
		}
	}

	return nil
}

// migrateDNSEntries copies the custom dns entries from the key-value dns
// table to dns_v1, resolving their networks by name within their tenants.
// Entries of networks that no longer exist are skipped. The old table is left
// as is.
func migrateDNSEntries(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(&types.DNSRecord{}) {
		return nil
	}

	var records []types.DNSRecord
	if err := db.FromContext(ctx).Find(&records).Error; err != nil {
		return err
	}

	networks := make(networkIDs)
	for _, record := range records {
		entry := record.Value.Data()
		networkID, err := networks.get(ctx, record.TenantID, entry.Network)
		if err != nil {
			return err
		}
		if networkID == "" {
			logger.Log(0, fmt.Sprintf(
				"skipping migration of dns entry %s: network %s (tenant %s) not found",
				entry.Name, entry.Network, record.TenantID,
			))
			continue
		}

		_entry := &schema.DNSEntry{
			ID:        uuid.NewString(),
			TenantID:  record.TenantID,
			NetworkID: networkID,
			Name:      entry.Name,
			Address:   entry.Address,
			Address6:  entry.Address6,
		}
		if err := _entry.Create(ctx); err != nil {
			return fmt.Errorf("failed to migrate dns entry %s of network %s (tenant %s): %w",
				entry.Name, entry.Network, record.TenantID, err)
		}
	}

	return nil
}

// migrateUserGroups populates the slugs of the user groups from their tenant
// prefixed ids, and replaces the ids with uuids. The ids of custom groups are
// already uuids, and are kept.
func migrateUserGroups(ctx context.Context) error {
	var groups []struct {
		ID       string
		TenantID string
	}
	err := db.FromContext(ctx).Model(&schema.UserGroup{}).
		Where("slug IS NULL").
		Select("id", "tenant_id").
		Find(&groups).Error
	if err != nil {
		return err
	}

	for _, group := range groups {
		slug := unscopedKey(group.TenantID, group.ID)
		err := db.FromContext(ctx).Model(&schema.UserGroup{}).
			Where("id = ?", group.ID).
			Updates(map[string]any{"id": idForSlug(slug), "slug": slug}).Error
		if err != nil {
			return fmt.Errorf("failed to migrate user group %s: %w", group.ID, err)
		}
	}

	return nil
}

// migrateUserRoles populates the slugs and scopes of the user roles from
// their ids, and replaces the ids with uuids. Network roles were tenant
// prefixed and are scoped to their tenants, the other roles are global.
func migrateUserRoles(ctx context.Context) error {
	var roles []struct {
		ID        string
		NetworkID string
	}
	err := db.FromContext(ctx).Model(&schema.UserRole{}).
		Where("slug IS NULL").
		Select("id", "network_id").
		Find(&roles).Error
	if err != nil {
		return err
	}

	for _, role := range roles {
		slug, roleScope, scopeID := role.ID, scope.GlobalScope, ""
		if role.NetworkID != "" {
			roleScope = scope.TenantScope
			if tenantID, logicalID, ok := strings.Cut(role.ID, "::"); ok {
				scopeID, slug = tenantID, logicalID
			}
		}

		err := db.FromContext(ctx).Model(&schema.UserRole{}).
			Where("id = ?", role.ID).
			Updates(map[string]any{
				"id":       idForSlug(slug),
				"slug":     slug,
				"scope":    roleScope,
				"scope_id": scopeID,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to migrate user role %s: %w", role.ID, err)
		}
	}

	return nil
}
