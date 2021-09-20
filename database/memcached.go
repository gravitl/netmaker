package database

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gravitl/netmaker/servercfg"
)

var MemCachedDatabase *memcache.Client

var MEMCACHED_FUNCTIONS = map[string]interface{}{
	INIT_DB:      initMemcachedDatabase,
	CREATE_TABLE: memcachedCreateTable,
	INSERT:       memcachedInsert,
	INSERT_PEER:  memcachedInsertPeer,
	DELETE:       memcachedDeleteRecord,
	DELETE_ALL:   memcachedDeleteAllRecords,
	FETCH_ALL:    memcachedFetchRecords,
	CLOSE_DB:     memcachedCloseDB,
}

// utility function to make setting memcached servers easier
func parseMemcachedAddresses(addresses string) string {
	addressesArr := strings.Split(addresses, ",")
	numAddresses := len(addressesArr)
	if numAddresses == 0 {
		return "127.0.0.1:11211"
	}
	newAddresses := ""
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	for _, address := range addressesArr {
		if isValidIp(address) {
			newAddresses += address
			if servercfg.GetVerbose() >= 2 {
				log.Println("adding " + address + " to memcached servers")
			}
			if address != addressesArr[numAddresses-1] {
				newAddresses += ","
			}
		}
	}
	return newAddresses
}

func initMemcachedDatabase() error {
	addresses := parseMemcachedAddresses(servercfg.GetMemcachedAddress())
	MemCachedDatabase = memcache.New(addresses)
	if MemCachedDatabase == nil {
		return errors.New("could not initialize memcached")
	}
	MemCachedDatabase.Timeout = time.Minute
	return nil
}

func memcachedCreateTable(tableName string) error {

	if currentTable, err := memcachedFetchRecords(tableName); (currentTable != nil && len(currentTable) >= 0) || err != nil {
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
	err = MemCachedDatabase.Set(&memcache.Item{Key: tableName, Value: newTable, Expiration: 0})
	if err != nil {
		return err
	}
	return nil
}

func memcachedInsert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		preData, err := MemCachedDatabase.Get(tableName)
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
		err = MemCachedDatabase.Replace(&memcache.Item{Key: tableName, Value: postData, Expiration: 0})
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

func memcachedInsertPeer(key string, value string) error {
	if key != "" && value != "" && IsJSONString(value) {
		if err := memcachedInsert(key, value, PEERS_TABLE_NAME); err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid peer insert " + key + " : " + value)
	}
}

func memcachedDeleteRecord(tableName string, key string) error {
	if key != "" {
		preData, err := MemCachedDatabase.Get(tableName)
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
		err = MemCachedDatabase.Set(&memcache.Item{Key: tableName, Value: postData, Expiration: 0})
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid delete, key is required")
	}
}

func memcachedDeleteAllRecords(tableName string) error {
	err := MemCachedDatabase.Delete(tableName)
	if err != nil {
		return err
	}
	err = memcachedCreateTable(tableName)
	if err != nil {
		return err
	}
	return nil
}

// func memcachedFetchRecord(tableName string, key string) (string, error) {
// 	results, err := memcachedFetchRecords(tableName)
// 	if err != nil {
// 		return "", err
// 	}
// 	if results[key] == "" {
// 		return "", errors.New(NO_RECORD)
// 	}
// 	return results[key], nil
// }

func memcachedFetchRecords(tableName string) (map[string]string, error) {
	var records map[string]string
	item, err := MemCachedDatabase.Get(tableName)
	if err != nil {
		return records, err
	}
	if err = json.Unmarshal(item.Value, &records); err != nil {
		return nil, err
	}

	return records, nil
}

func memcachedCloseDB() {
	// no op for this library..
}

func isValidIp(ipAddr string) bool {
	return net.ParseIP(ipAddr) == nil
}
