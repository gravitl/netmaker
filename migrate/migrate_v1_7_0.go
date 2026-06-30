package migrate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TableName_ServerConf     = "serverconf"
	TableName_Generated      = "generated"
	TableName_ServerUUID     = "serveruuid"
	TableName_EnrollmentKey  = "enrollmentkeys"
	TableName_ServerSettings = "server_settings"
)

const (
	LegacyServerSettingsKey = "server_cfg"
)

func migrateV1_7_0(ctx context.Context) error {
	err := migrateServerConf(ctx)
	if err != nil {
		return err
	}

	err = migrateGenerated(ctx)
	if err != nil {
		return err
	}

	err = migrateServerUUID(ctx)
	if err != nil {
		return err
	}

	err = migrateEnrollmentKeys(ctx)
	if err != nil {
		return err
	}

	err = migrateServerSettings(ctx)
	if err != nil {
		return err
	}

	err = createMemberships(ctx)
	if err != nil {
		return err
	}

	err = setTenantID(ctx)
	if err != nil {
		return err
	}

	return setNetworkID(ctx)
}

func migrateServerConf(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(TableName_ServerConf) {
		return nil
	}

	records, err := kvList(ctx, TableName_ServerConf)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	record, ok := records["nm-jwt-secret"]
	if ok {
		recordData := make(map[string]string)
		err = json.Unmarshal([]byte(record), &recordData)
		if err != nil {
			return err
		}

		jwtSecretValue, ok := recordData["privatekey"]
		if ok {
			jwtSecret := &schema.Internal{
				Key:   schema.InternalKey_JwtSecret,
				Value: jwtSecretValue,
			}
			err = jwtSecret.Set(ctx)
			if err != nil {
				return err
			}
		}
	}

	record, ok = records["netmaker-id-key-pair"]
	if ok {
		recordData := make(map[string][]byte)
		err = json.Unmarshal([]byte(record), &recordData)
		if err != nil {
			return err
		}

		privateKeyValue, ok := recordData["private_key"]
		if ok {
			privateKey := &schema.Internal{
				Key:   schema.InternalKey_LicenseValidationPrivateKey,
				Value: base64.StdEncoding.EncodeToString(privateKeyValue),
			}
			err = privateKey.Set(ctx)
			if err != nil {
				return err
			}
		}

		publicKeyValue, ok := recordData["public_key"]
		if ok {
			publicKey := &schema.Internal{
				Key:   schema.InternalKey_LicenseValidationPublicKey,
				Value: base64.StdEncoding.EncodeToString(publicKeyValue),
			}
			err = publicKey.Set(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateGenerated(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(TableName_Generated) {
		return nil
	}

	records, err := kvList(ctx, TableName_Generated)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	record, ok := records["netmaker_auth"]
	if ok {
		recordData := make(map[string]string)
		err = json.Unmarshal([]byte(record), &recordData)
		if err != nil {
			return err
		}

		oauthSecretValue, ok := recordData["value"]
		if ok {
			oauthSecret := &schema.Internal{
				Key:   schema.InternalKey_OAuthSecret,
				Value: oauthSecretValue,
			}
			err = oauthSecret.Set(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateServerUUID(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(TableName_ServerUUID) {
		return nil
	}

	records, err := kvList(ctx, TableName_ServerUUID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	record, ok := records["serveruuid"]
	if ok {
		type recordType struct {
			UUID           string `json:"uuid"`
			LastSend       int64  `json:"lastsend"`
			TrafficKeyPriv []byte `json:"traffickeypriv"`
			TrafficKeyPub  []byte `json:"traffickeypub"`
		}

		var recordData recordType
		err = json.Unmarshal([]byte(record), &recordData)
		if err != nil {
			return err
		}

		if recordData.UUID != "" {
			serverID := &schema.Internal{
				Key:   schema.InternalKey_ServerID,
				Value: recordData.UUID,
			}
			err = serverID.Set(ctx)
			if err != nil {
				return err
			}
		}

		if recordData.LastSend != 0 {
			telemetryLastReportedAt := &schema.Internal{
				Key:   schema.InternalKey_TelemetryLastReportedAt,
				Value: time.Unix(recordData.LastSend, 0).UTC().Format(time.RFC3339),
			}
			err = telemetryLastReportedAt.Set(ctx)
			if err != nil {
				return err
			}
		}

		if recordData.TrafficKeyPriv != nil && recordData.TrafficKeyPub != nil {
			mqPrivateKey := &schema.Internal{
				Key:   schema.InternalKey_MqPrivateKey,
				Value: base64.StdEncoding.EncodeToString(recordData.TrafficKeyPriv),
			}
			err = mqPrivateKey.Set(ctx)
			if err != nil {
				return err
			}

			mqPublicKey := &schema.Internal{
				Key:   schema.InternalKey_MqPublicKey,
				Value: base64.StdEncoding.EncodeToString(recordData.TrafficKeyPub),
			}
			err = mqPublicKey.Set(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateEnrollmentKeys(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(TableName_EnrollmentKey) {
		return nil
	}

	records, err := kvList(ctx, TableName_EnrollmentKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	for _, record := range records {
		var key models.EnrollmentKey
		if err = json.Unmarshal([]byte(record), &key); err != nil {
			return err
		}

		tags := make(datatypes.JSONSlice[string], 0, len(key.Groups))
		for _, g := range key.Groups {
			tags = append(tags, g.String())
		}

		var gatewayID *string
		if key.Relay != uuid.Nil {
			s := key.Relay.String()
			gatewayID = &s
		}

		var keyType schema.EnrollmentKeyType
		switch key.Type {
		case models.Unlimited:
			keyType = schema.EnrollmentKeyType_UnlimitedUses
		case models.Uses:
			keyType = schema.EnrollmentKeyType_LimitedUses
		case models.TimeExpiration:
			keyType = schema.EnrollmentKeyType_TimedExpiry
		default:
			keyType = schema.EnrollmentKeyType_UnlimitedUses
		}

		// models.Tags[0] was used as the enrollment key display name
		name := ""
		if len(key.Tags) > 0 {
			name = key.Tags[0]
		}

		_key := &schema.EnrollmentKey{
			ID:                uuid.NewString(),
			Name:              name,
			Value:             key.Value,
			Token:             key.Token,
			Default:           key.Default,
			Unlimited:         key.Unlimited,
			UsesRemaining:     key.UsesRemaining,
			Expiration:        key.Expiration,
			Networks:          key.Networks,
			Tags:              tags,
			GatewayID:         gatewayID,
			AutoEgress:        key.AutoEgress,
			AutoAssignGateway: key.AutoAssignGateway,
			Type:              keyType,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		if err = _key.Create(ctx); err != nil {
			return err
		}
	}

	return nil
}

func migrateServerSettings(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(TableName_ServerSettings) {
		return nil
	}

	records, err := kvList(ctx, TableName_ServerSettings)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	defaultTenant := &schema.Tenant{}
	err = defaultTenant.GetDefault(ctx)
	if err != nil {
		return err
	}

	for key, value := range records {
		if key == LegacyServerSettingsKey {
			err = kvInsert(ctx, TableName_ServerSettings, defaultTenant.ID, json.RawMessage(value))
			if err != nil {
				return err
			}

			err = kvDelete(ctx, TableName_ServerSettings, LegacyServerSettingsKey)
			if err != nil {
				return err
			}
		} else {
			var userSettings models.UserSettings
			err = json.Unmarshal([]byte(value), &userSettings)
			if err != nil {
				return err
			}

			user := schema.User{Username: key}
			err = user.Get(ctx)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					err = kvDelete(ctx, TableName_ServerSettings, key)
					if err != nil {
						return err
					}
					continue
				} else {
					return err
				}
			}

			user.Theme = userSettings.Theme
			user.TextSize = userSettings.TextSize
			user.ReducedMotion = userSettings.ReducedMotion
			err = user.UpdateUserSettings(ctx)
			if err != nil {
				return err
			}

			err = kvDelete(ctx, TableName_ServerSettings, key)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createMemberships(ctx context.Context) error {
	defaultOrg := &schema.Organization{}
	err := defaultOrg.GetDefault(ctx)
	if err != nil {
		return err
	}

	defaultTenant := &schema.Tenant{}
	err = defaultTenant.GetDefault(ctx)
	if err != nil {
		return err
	}

	var legacyUsers []types.LegacyUser
	err = db.FromContext(ctx).Find(&legacyUsers).Error
	if err != nil {
		return err
	}

	for _, u := range legacyUsers {
		tm := &schema.TenantMembership{
			TenantID:                   defaultTenant.ID,
			UserID:                     u.ID,
			RoleID:                     u.PlatformRoleID,
			Groups:                     u.UserGroups,
			AuthType:                   u.AuthType,
			ExternalIdentityProviderID: u.ExternalIdentityProviderID,
			Password:                   u.Password,
		}
		err = db.FromContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}). // conflicts can happen if migrating from version < v1.5.1
			Create(tm).Error
		if err != nil {
			return err
		}

		if u.PlatformRoleID == schema.SuperAdminRole {
			om := &schema.OrgMembership{
				OrganizationID: defaultOrg.ID,
				UserID:         u.ID,
				RoleID:         schema.OrgOwner,
			}
			err = db.FromContext(ctx).
				Clauses(clause.OnConflict{DoNothing: true}). // conflicts can happen if migrating from version < v1.5.1
				Create(om).Error
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func setTenantID(ctx context.Context) error {
	defaultTenant := &schema.Tenant{}
	err := defaultTenant.GetDefault(ctx)
	if err != nil {
		return err
	}

	tenantModels := []any{
		&schema.AclRecord{}, &schema.CacheRecord{}, &schema.DNSRecord{}, &schema.Nameserver{},
		&schema.Egress{}, &schema.EnrollmentKey{}, &schema.Event{}, &schema.ExtClientRecord{},
		&schema.Host{}, &schema.Integration{}, &schema.Internal{}, &schema.JITGrant{}, &schema.JITRequest{},
		&schema.MetricsRecord{}, &schema.Network{}, &schema.Node{}, &schema.PendingHost{}, &schema.PendingUser{},
		&schema.PostureCheck{}, &schema.PostureCheckViolation{}, &schema.SsoStateRecord{},
		&schema.TagRecord{}, &schema.UserAccessToken{}, &schema.UserGroup{}, &schema.UserInvite{},
	}

	for _, model := range tenantModels {
		err := db.FromContext(ctx).Model(model).
			Where("tenant_id = ?", "").
			Update("tenant_id", defaultTenant.ID).
			Error
		if err != nil {
			return err
		}
	}

	return nil
}

func setNetworkID(ctx context.Context) error {
	var aclRecords []schema.AclRecord
	err := db.FromContext(ctx).Find(&aclRecords).Error
	if err != nil {
		return err
	}

	for _, record := range aclRecords {
		err := db.FromContext(ctx).Model(&record).
			Update("network_id", string(record.Value.Data().NetworkID)).
			Error
		if err != nil {
			return err
		}
	}

	var dnsRecords []schema.DNSRecord
	err = db.FromContext(ctx).Find(&dnsRecords).Error
	if err != nil {
		return err
	}

	for _, record := range dnsRecords {
		err := db.FromContext(ctx).Model(&record).
			Update("network_id", record.Value.Data().Network).
			Error
		if err != nil {
			return err
		}
	}

	var extClientRecords []schema.ExtClientRecord
	err = db.FromContext(ctx).Find(&extClientRecords).Error
	if err != nil {
		return err
	}

	for _, record := range extClientRecords {
		err := db.FromContext(ctx).Model(&record).
			Update("network_id", record.Value.Data().Network).
			Error
		if err != nil {
			return err
		}
	}

	var tagRecords []schema.TagRecord
	err = db.FromContext(ctx).Find(&tagRecords).Error
	if err != nil {
		return err
	}

	for _, record := range tagRecords {
		err := db.FromContext(ctx).Model(&record).
			Update("network_id", string(record.Value.Data().Network)).
			Error
		if err != nil {
			return err
		}
	}

	var metricsRecords []schema.MetricsRecord
	err = db.FromContext(ctx).Find(&metricsRecords).Error
	if err != nil {
		return err
	}

	for _, record := range metricsRecords {
		err := db.FromContext(ctx).Model(&record).
			Update("network_id", record.Value.Data().Network).
			Error
		if err != nil {
			return err
		}
	}

	return nil
}
