package database

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/gravitl/netmaker/servercfg"
	"google.golang.org/appengine/memcache"
)

var EtcdDatabase *clientv3.Client
var KV *clientv3.KV

var ETCD_FUNCTIONS = map[string]interface{}{
	INIT_DB:      initEtcdDatabase,
	CREATE_TABLE: etcdCreateTable,
	INSERT:       etcdInsert,
	INSERT_PEER:  etcdInsertPeer,
	DELETE:       etcdDeleteRecord,
	DELETE_ALL:   etcdDeleteAllRecords,
	FETCH_ALL:    etcdFetchRecords,
	CLOSE_DB:     etcdCloseDB,
}

// utility function to make setting etcd servers easier
func parseEtcdAddresses(addresses string) string {
	addressesArr := strings.Split(addresses, ",")
	numAddresses := len(addressesArr)
	if numAddresses == 0 {
		return "127.0.0.1:2379"
	}
	newAddresses := ""
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	for _, address := range addressesArr {
		if isValidIp(address) {
			newAddresses += address
			if servercfg.GetVerbose() >= 2 {
				log.Println("adding " + address + " to etcd servers")
			}
			if address != addressesArr[numAddresses-1] {
				newAddresses += ","
			}
		}
	}
	return newAddresses
}

func initEtcdDatabase() error {
	addresses := parseEtcdAddresses(servercfg.GetEtcdAddresses())
	var err error
	EtcdDatabase, err = clientv3.New(clientv3.Config{
		Endpoints:   []string{addresses},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	} else if EtcdDatabase == nil {
		return errors.New("could not initialize etcd")
	}
	EtcdDatabase.Timeout = time.Minute
	clientv3.NewKV(EtcdDatabase)
	return nil
}

func etcdCreateTable(tableName string) error {

	if currentTable, err := etcdFetchRecords(tableName); (currentTable != nil && len(currentTable) >= 0) || err != nil {
		// return if it already exists
		return err
	} else {
		log.Println(currentTable)
	}
	table := make(map[string]string)
	newTable, err := json.Marshal(table)
	if err != nil {
		return err
	}
	kv := clientv3.NewKV(EtcdDatabase)
	err = EtcdDatabase.Set(&memcache.Item{Key: tableName, Value: newTable, Expiration: 0})
	if err != nil {
		return err
	}
	return nil
}

func etcdInsert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		preData, err := EtcdDatabase.Get(tableName)
		if err != nil {
			return err
		}
		var preDataMap map[string]string
		if err := json.Unmarshal(preData.Value, &preDataMap); err != nil {
			return err
		}
		preDataMap[key] = value
		postData, err := json.Marshal(&preDataMap)
		if err != nil {
			return err
		}
		err = EtcdDatabase.Replace(&memcache.Item{Key: tableName, Value: postData, Expiration: 0})
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

func etcdInsertPeer(key string, value string) error {
	if key != "" && value != "" && IsJSONString(value) {
		if err := etcdInsert(key, value, PEERS_TABLE_NAME); err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid peer insert " + key + " : " + value)
	}
}

func etcdDeleteRecord(tableName string, key string) error {
	if key != "" {
		preData, err := EtcdDatabase.Get(tableName)
		if err != nil {
			return err
		}
		var preDataMap map[string]string
		if err := json.Unmarshal(preData.Value, &preDataMap); err != nil {
			return err
		}
		delete(preDataMap, key)
		postData, err := json.Marshal(&preDataMap)
		if err != nil {
			return err
		}
		err = EtcdDatabase.Set(&memcache.Item{Key: tableName, Value: postData, Expiration: 0})
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid delete, key is required")
	}
}

func etcdDeleteAllRecords(tableName string) error {
	err := EtcdDatabase.Delete(tableName)
	if err != nil {
		return err
	}
	err = etcdCreateTable(tableName)
	if err != nil {
		return err
	}
	return nil
}

// func etcdFetchRecord(tableName string, key string) (string, error) {
// 	results, err := etcdFetchRecords(tableName)
// 	if err != nil {
// 		return "", err
// 	}
// 	if results[key] == "" {
// 		return "", errors.New(NO_RECORD)
// 	}
// 	return results[key], nil
// }

func etcdFetchRecords(tableName string) (map[string]string, error) {
	var records map[string]string
	item, err := EtcdDatabase.Get(tableName)
	if err != nil {
		return records, err
	}
	if err = json.Unmarshal(item.Value, &records); err != nil {
		return nil, err
	}

	return records, nil
}

func etcdCloseDB() {
	// no op for this library..
}

func isValidIp(ipAddr string) bool {
	return net.ParseIP(ipAddr) == nil
}
