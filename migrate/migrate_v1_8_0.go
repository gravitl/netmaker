package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const TableName_ExtClients = "extclients"

func migrateV1_8_0(ctx context.Context) error {
	err := migrateExtClientsToSQLTable(ctx)
	if err != nil {
		return err
	}

	return migrateUserRolesToScoped(ctx)
}

func migrateExtClientsToSQLTable(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(TableName_ExtClients) {
		return nil
	}

	var records []types.ExtClientRecord
	if err := db.FromContext(ctx).Find(&records).Error; err != nil {
		return err
	}

	for _, record := range records {
		extclient := record.Value.Data()

		v1 := &schema.Extclient{
			ID:        uuid.NewString(),
			TenantID:  record.TenantID,
			NetworkID: record.NetworkID,
			Name:      extclient.ClientID,

			PrivateKey: extclient.PrivateKey,
			PublicKey:  extclient.PublicKey,
			DNS:        extclient.DNS,
			Address:    extclient.Address,
			Address6:   extclient.Address6,

			IngressGatewayID:       extclient.IngressGatewayID,
			IngressGatewayEndpoint: extclient.IngressGatewayEndpoint,

			AllowedIPs:      datatypes.JSONSlice[string](extclient.AllowedIPs),
			ExtraAllowedIPs: datatypes.JSONSlice[string](extclient.ExtraAllowedIPs),

			PostUp:   extclient.PostUp,
			PostDown: extclient.PostDown,

			SelectedInternetEgressID: extclient.SelectedInternetEgressID,
			Enabled:                  extclient.Enabled,
			OwnerID:                  extclient.OwnerID,

			Status:                      extclient.Status,
			PostureCheckSeverity:        extclient.PostureCheckVolationSeverityLevel,
			PostureCheckLastEvaluatedAt: extclient.LastEvaluatedAt,

			DeniedACLs:           datatypes.NewJSONType(extclient.DeniedACLs),
			RemoteAccessClientID: extclient.RemoteAccessClientID,
			Tags:                 datatypes.NewJSONType(extclient.Tags),
			OS:                   extclient.OS,
			OSFamily:             extclient.OSFamily,
			OSVersion:            extclient.OSVersion,
			KernelVersion:        extclient.KernelVersion,
			ClientVersion:        extclient.ClientVersion,
			DeviceID:             extclient.DeviceID,
			DeviceName:           extclient.DeviceName,
			PublicEndpoint:       extclient.PublicEndpoint,
			Country:              extclient.Country,
			Location:             extclient.Location,
			JITExpiresAt:         extclient.JITExpiresAt,
		}

		if extclient.LastModified != 0 {
			lastModified := time.Unix(extclient.LastModified, 0).UTC()
			v1.CreatedAt = lastModified
			v1.UpdatedAt = lastModified
		}

		if len(extclient.PostureChecksViolations) > 0 {
			cycleID := uuid.NewString()
			v1.PostureCheckLastEvaluationCycleID = cycleID

			violations := make([]schema.PostureCheckViolation, 0, len(extclient.PostureChecksViolations))
			for _, v := range extclient.PostureChecksViolations {
				violations = append(violations, schema.PostureCheckViolation{
					EvaluationCycleID: cycleID,
					TenantID:          record.TenantID,
					CheckID:           v.CheckID,
					NodeID:            v1.ID,
					SubjectType:       schema.PostureCheckSubjectType_ExtClient,
					Name:              v.Name,
					Attribute:         v.Attribute,
					Message:           v.Message,
					Severity:          v.Severity,
					EvaluatedAt:       extclient.LastEvaluatedAt,
				})
			}

			if err := db.FromContext(ctx).Model(&schema.PostureCheckViolation{}).Create(&violations).Error; err != nil {
				return err
			}
		}

		if err := db.FromContext(ctx).Model(&schema.Extclient{}).Create(v1).Error; err != nil {
			return err
		}
	}

	return nil
}

func migrateUserRolesToScoped(ctx context.Context) error {
	tx := db.FromContext(ctx)
	if !tx.Migrator().HasTable("user_roles_v1") {
		return nil
	}

	var rows []schema.UserRole
	if err := tx.Find(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		oldID := r.ID.String()
		update := map[string]interface{}{}

		if idx := strings.Index(oldID, "::"); idx != -1 {
			update["scope"] = scope.TenantScope
			update["scope_id"] = oldID[:idx]
			if r.Name == "" {
				logicalID := oldID[idx+2:]
				admin, err := schema.NetworkRoleTypeFromLegacyID(logicalID)
				if err != nil {
					return fmt.Errorf("migrate user role %q: %w", oldID, err)
				}
				update["name"] = schema.NetworkRoleDisplayName(r.NetworkID, admin)
			}
		} else if r.Default && r.Name == "" {
			update["name"] = oldID
			if orchestrator.ValidOrgRoles[r.ID] {
				update["scope"] = scope.OrgScope
			} else {
				update["scope"] = scope.TenantScope
			}
		} else {
			continue
		}

		update["id"] = uuid.NewString()
		if err := tx.Model(&schema.UserRole{}).Where("id = ?", oldID).Updates(update).Error; err != nil {
			return err
		}
	}

	return nil
}
