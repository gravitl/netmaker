package migrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func migrateV1_8_0(ctx context.Context) error {
	return migrateExtClients(ctx)
}

func migrateExtClients(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(&types.ExtClientRecord{}) {
		return nil
	}

	var records []types.ExtClientRecord
	if err := db.FromContext(ctx).Find(&records).Error; err != nil {
		return err
	}

	// tenant id -> network name -> network id
	networkIDs := make(map[string]map[string]string)
	for _, record := range records {
		extclient := record.Value.Data()
		if extclient.ClientID == "" {
			logger.Log(0, fmt.Sprintf("skipping migration of extclient record %s: no client id", record.Key))
			continue
		}

		if networkIDs[record.TenantID] == nil {
			networkIDs[record.TenantID] = make(map[string]string)
		}
		networkID, ok := networkIDs[record.TenantID][extclient.Network]
		if !ok {
			var network schema.Network
			err := db.FromContext(ctx).Model(&schema.Network{}).
				Where("tenant_id = ? AND name = ?", record.TenantID, extclient.Network).
				First(&network).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			networkID = network.ID
			networkIDs[record.TenantID][extclient.Network] = networkID
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
