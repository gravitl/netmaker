package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
)

const (
	TableName_DNS        = "dns"
	TableName_ExtClients = "extclients"
	TableName_Acls       = "acls"
	TableName_Metrics    = "metrics"
	TableName_Tags       = "tags"
)

// migrateMultitenancy bootstraps the multi-tenancy foundation:
//  1. Adds tenant_id (and network_id where applicable) columns to old KV tables.
//  2. Creates the default Organization and Tenant if none exist.
//  3. Backfills tenant_id and network_id on all existing KV table records.
//  4. Backfills tenant_id on all existing GORM resource records.
func migrateMultitenancy(ctx context.Context) error {
	err := addTenantColumnsToKVTables(ctx)
	if err != nil {
		return err
	}

	defaultOrg := &schema.Organization{}
	err = defaultOrg.CreateDefault(ctx)
	if err != nil {
		return fmt.Errorf("multitenancy migration: failed to create default organization: %w", err)
	}

	defaultTenant := &schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	err = defaultTenant.CreateDefault(ctx)
	if err != nil {
		return fmt.Errorf("multitenancy migration: failed to create default tenant: %w", err)
	}

	err = backfillKVTableTenantID(ctx, defaultTenant.ID)
	if err != nil {
		return err
	}

	err = backfillTenantID(ctx, defaultTenant.ID)
	if err != nil {
		return err
	}

	return backfillKVTableNetworkID(ctx)
}

// addTenantColumnsToKVTables runs ALTER TABLE on the legacy key-value tables to
// add tenant_id and network_id columns. Errors caused by the column already
// existing are ignored so the migration is safe to re-run.
func addTenantColumnsToKVTables(ctx context.Context) error {
	withNetworkID := []string{TableName_DNS, TableName_ExtClients, TableName_Acls, TableName_Metrics, TableName_Tags}
	for _, table := range withNetworkID {
		if err := addColumnIfNotExists(ctx, table, "tenant_id", "TEXT"); err != nil {
			return fmt.Errorf("multitenancy migration: alter table %s (tenant_id): %w", table, err)
		}
		if err := addColumnIfNotExists(ctx, table, "network_id", "TEXT"); err != nil {
			return fmt.Errorf("multitenancy migration: alter table %s (network_id): %w", table, err)
		}
	}

	tenantOnly := []string{"server_settings"}
	for _, table := range tenantOnly {
		if err := addColumnIfNotExists(ctx, table, "tenant_id", "TEXT"); err != nil {
			return fmt.Errorf("multitenancy migration: alter table %s (tenant_id): %w", table, err)
		}
	}

	return nil
}

// addColumnIfNotExists attempts ALTER TABLE … ADD COLUMN and ignores the error
// if the column already exists (SQLite: "duplicate column name", PostgreSQL: "already exists").
func addColumnIfNotExists(ctx context.Context, table, column, colDef string) error {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colDef)
	err := db.FromContext(ctx).Exec(sql).Error
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists") {
		return nil
	}
	return err
}

// backfillKVTableTenantID sets tenant_id = tenantID on every row in the legacy
// KV tables where tenant_id is NULL or empty.
func backfillKVTableTenantID(ctx context.Context, tenantID string) error {
	tables := []string{TableName_DNS, TableName_ExtClients, TableName_Acls, TableName_Metrics, TableName_Tags, "server_settings"}
	for _, table := range tables {
		sql := fmt.Sprintf("UPDATE %s SET tenant_id = ? WHERE tenant_id IS NULL OR tenant_id = ''", table)
		if err := db.FromContext(ctx).Exec(sql, tenantID).Error; err != nil {
			return fmt.Errorf("multitenancy migration: backfill tenant_id in %s: %w", table, err)
		}
	}
	return nil
}

// backfillKVTableNetworkID unmarshals each legacy KV table row into its typed
// model and writes the network identifier into the new network_id column.
func backfillKVTableNetworkID(ctx context.Context) error {
	if err := backfillDNSNetworkID(ctx); err != nil {
		return err
	}
	if err := backfillExtClientNetworkID(ctx); err != nil {
		return err
	}
	if err := backfillAclNetworkID(ctx); err != nil {
		return err
	}
	if err := backfillMetricsNetworkID(ctx); err != nil {
		return err
	}
	return backfillTagNetworkID(ctx)
}

func backfillDNSNetworkID(ctx context.Context) error {
	records, err := kvList(ctx, TableName_DNS)
	if err != nil {
		return fmt.Errorf("multitenancy migration: list dns records: %w", err)
	}
	for key, value := range records {
		var entry schema.DNSEntry
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			return fmt.Errorf("multitenancy migration: parse dns record %s: %w", key, err)
		}
		if entry.Network == "" {
			continue
		}
		logger.Log(4, fmt.Sprintf("multitenancy migration: backfilling network_id for dns record %s", key))
		if err := db.FromContext(ctx).Exec(
			"UPDATE dns SET network_id = ? WHERE key = ? AND (network_id IS NULL OR network_id = '')",
			entry.Network, key,
		).Error; err != nil {
			return fmt.Errorf("multitenancy migration: set network_id in dns record %s: %w", key, err)
		}
	}
	return nil
}

func backfillExtClientNetworkID(ctx context.Context) error {
	records, err := kvList(ctx, TableName_ExtClients)
	if err != nil {
		return fmt.Errorf("multitenancy migration: list extclients records: %w", err)
	}
	for key, value := range records {
		var client schema.ExtClient
		if err := json.Unmarshal([]byte(value), &client); err != nil {
			return fmt.Errorf("multitenancy migration: parse extclient record %s: %w", key, err)
		}
		if client.Network == "" {
			continue
		}
		logger.Log(4, fmt.Sprintf("multitenancy migration: backfilling network_id for extclient record %s", key))
		if err := db.FromContext(ctx).Exec(
			"UPDATE extclients SET network_id = ? WHERE key = ? AND (network_id IS NULL OR network_id = '')",
			client.Network, key,
		).Error; err != nil {
			return fmt.Errorf("multitenancy migration: set network_id in extclient record %s: %w", key, err)
		}
	}
	return nil
}

func backfillAclNetworkID(ctx context.Context) error {
	records, err := kvList(ctx, TableName_Acls)
	if err != nil {
		return fmt.Errorf("multitenancy migration: list acls records: %w", err)
	}
	for key, value := range records {
		var acl schema.Acl
		if err := json.Unmarshal([]byte(value), &acl); err != nil {
			return fmt.Errorf("multitenancy migration: parse acl record %s: %w", key, err)
		}
		if acl.NetworkID == "" {
			continue
		}
		logger.Log(4, fmt.Sprintf("multitenancy migration: backfilling network_id for acl record %s", key))
		if err := db.FromContext(ctx).Exec(
			"UPDATE acls SET network_id = ? WHERE key = ? AND (network_id IS NULL OR network_id = '')",
			string(acl.NetworkID), key,
		).Error; err != nil {
			return fmt.Errorf("multitenancy migration: set network_id in acl record %s: %w", key, err)
		}
	}
	return nil
}

func backfillMetricsNetworkID(ctx context.Context) error {
	records, err := kvList(ctx, TableName_Metrics)
	if err != nil {
		return fmt.Errorf("multitenancy migration: list metrics records: %w", err)
	}
	for key, value := range records {
		var m schema.Metrics
		if err := json.Unmarshal([]byte(value), &m); err != nil {
			return fmt.Errorf("multitenancy migration: parse metrics record %s: %w", key, err)
		}
		if m.Network == "" {
			continue
		}
		logger.Log(4, fmt.Sprintf("multitenancy migration: backfilling network_id for metrics record %s", key))
		if err := db.FromContext(ctx).Exec(
			"UPDATE metrics SET network_id = ? WHERE key = ? AND (network_id IS NULL OR network_id = '')",
			m.Network, key,
		).Error; err != nil {
			return fmt.Errorf("multitenancy migration: set network_id in metrics record %s: %w", key, err)
		}
	}
	return nil
}

func backfillTagNetworkID(ctx context.Context) error {
	records, err := kvList(ctx, TableName_Tags)
	if err != nil {
		return fmt.Errorf("multitenancy migration: list tags records: %w", err)
	}
	for key, value := range records {
		var tag schema.Tag
		if err := json.Unmarshal([]byte(value), &tag); err != nil {
			return fmt.Errorf("multitenancy migration: parse tag record %s: %w", key, err)
		}
		if tag.Network == "" {
			continue
		}
		logger.Log(4, fmt.Sprintf("multitenancy migration: backfilling network_id for tag record %s", key))
		if err := db.FromContext(ctx).Exec(
			"UPDATE tags SET network_id = ? WHERE key = ? AND (network_id IS NULL OR network_id = '')",
			string(tag.Network), key,
		).Error; err != nil {
			return fmt.Errorf("multitenancy migration: set network_id in tag record %s: %w", key, err)
		}
	}
	return nil
}

// backfillTenantID updates all existing GORM resource records that have an
// empty tenant_id to the provided default tenant ID.
func backfillTenantID(ctx context.Context, tenantID string) error {
	gormDB := db.FromContext(ctx)

	type namedModel struct {
		name  string
		model any
	}

	gormModels := []namedModel{
		{"Network", &schema.Network{}},
		{"Host", &schema.Host{}},
		{"Node", &schema.Node{}},
		{"EnrollmentKey", &schema.EnrollmentKey{}},
		{"Egress", &schema.Egress{}},
		{"PendingHost", &schema.PendingHost{}},
		{"PendingUser", &schema.PendingUser{}},
		{"UserInvite", &schema.UserInvite{}},
		{"UserGroup", &schema.UserGroup{}},
		{"JITRequest", &schema.JITRequest{}},
		{"JITGrant", &schema.JITGrant{}},
		{"PostureCheck", &schema.PostureCheck{}},
		{"PostureCheckViolation", &schema.PostureCheckViolation{}},
		{"Integration", &schema.Integration{}},
		{"Event", &schema.Event{}},
		{"Job", &schema.Job{}},
		{"UserAccessToken", &schema.UserAccessToken{}},
		{"Nameserver", &schema.Nameserver{}},
	}

	for _, m := range gormModels {
		logger.Log(4, fmt.Sprintf("multitenancy migration: backfilling tenant_id for %s", m.name))
		if err := gormDB.Model(m.model).Where("tenant_id = ''").Update("tenant_id", tenantID).Error; err != nil {
			return fmt.Errorf("multitenancy migration: backfill tenant_id for %s: %w", m.name, err)
		}
	}
	return nil
}
