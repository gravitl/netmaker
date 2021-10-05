package controller

import (
	"encoding/json"
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/dnslogic"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

/**
 * If being deleted by server, create a record in the DELETED_NODES_TABLE for the client to find
 * If being deleted by the client, delete completely
 */
func DeleteNode(key string, exterminate bool) error {
	var err error
	if !exterminate {
		args := strings.Split(key, "###")
		node, err := GetNode(args[0], args[1])
		if err != nil {
			return err
		}
		node.Action = models.NODE_DELETE
		nodedata, err := json.Marshal(&node)
		if err != nil {
			return err
		}
		err = database.Insert(key, string(nodedata), database.DELETED_NODES_TABLE_NAME)
		if err != nil {
			return err
		}
	} else {
		if err := database.DeleteRecord(database.DELETED_NODES_TABLE_NAME, key); err != nil {
			functions.PrintUserLog("", err.Error(), 2)
		}
	}
	if err := database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		return err
	}
	if servercfg.IsDNSMode() {
		err = dnslogic.SetDNS()
	}
	return err
}

func DeleteIntClient(clientid string) (bool, error) {

	err := database.DeleteRecord(database.INT_CLIENTS_TABLE_NAME, clientid)
	if err != nil {
		return false, err
	}

	return true, nil
}

func GetNode(macaddress string, network string) (models.Node, error) {

	var node models.Node

	key, err := functions.GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}
	data, err := database.FetchRecord(database.NODES_TABLE_NAME, key)
	if err != nil {
		if data == "" {
			data, err = database.FetchRecord(database.DELETED_NODES_TABLE_NAME, key)
			err = json.Unmarshal([]byte(data), &node)
		}
		return node, err
	}
	if err = json.Unmarshal([]byte(data), &node); err != nil {
		return node, err
	}
	node.SetDefaults()

	return node, err
}

func GetIntClient(clientid string) (models.IntClient, error) {

	var client models.IntClient

	value, err := database.FetchRecord(database.INT_CLIENTS_TABLE_NAME, clientid)
	if err != nil {
		return client, err
	}
	if err = json.Unmarshal([]byte(value), &client); err != nil {
		return models.IntClient{}, err
	}
	return client, nil
}
