package migrate

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

// ToSQLSchema migrates the data from key-value
// db to sql db.
//
// This function archives the old data and does not
// delete it.
func ToSQLSchema() error {
	// begin a new transaction.
	dbctx := db.BeginTx(context.TODO())
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	// check if migrated already.
	migrationJob := &schema.Job{
		ID: "migration-v1.0.0",
	}
	err := migrationJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// migrate.
		err = migrateNetworks(dbctx)
		if err != nil {
			return err
		}

		err = migrateHosts(dbctx)
		if err != nil {
			return err
		}

		err = migrateNodes(dbctx)
		if err != nil {
			return err
		}

		err = migrateACLs(dbctx)
		if err != nil {
			return err
		}

		// mark migration job completed.
		err = migrationJob.Create(dbctx)
		if err != nil {
			return err
		}

		commit = true
	}

	return nil
}

func migrateNetworks(ctx context.Context) error {
	networks, err := database.FetchRecords(database.NETWORKS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, networkJson := range networks {
		var network models.Network
		err = json.Unmarshal([]byte(networkJson), &network)
		if err != nil {
			return err
		}

		_network := converters.ToSchemaNetwork(network)
		err = _network.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func migrateHosts(ctx context.Context) error {
	hosts, err := database.FetchRecords(database.HOSTS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, hostJson := range hosts {
		var host models.Host
		err = json.Unmarshal([]byte(hostJson), &host)
		if err != nil {
			return err
		}

		_host := converters.ToSchemaHost(host)
		err = _host.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func migrateNodes(ctx context.Context) error {
	nodes, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, nodeJson := range nodes {
		var node models.Node
		err = json.Unmarshal([]byte(nodeJson), &node)
		if err != nil {
			return err
		}

		_node := converters.ToSchemaNode(node)
		err = _node.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func migrateACLs(ctx context.Context) error {
	acls, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, aclJson := range acls {
		var acl models.Acl
		err = json.Unmarshal([]byte(aclJson), &acl)
		if err != nil {
			return err
		}

		_acl := converters.ToSchemaACL(acl)
		err = _acl.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
