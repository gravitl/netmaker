package database

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"time"
	"context"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"github.com/gravitl/netmaker/servercfg"
)

var EtcdDatabase *clientv3.Client

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
		DialTimeout: 30 * time.Second,
	})
	if servercfg.IsEtcdSSL() {
		tlsInfo := transport.TLSInfo{
			KeyFile:        servercfg.GetEtcdKeyPath(),
			CertFile:       servercfg.GetEtcdCertPath(),
			TrustedCAFile:  servercfg.GetEtcdCACertPath(),
			ClientCertAuth: true,
		}
		tlsConfig, errN := tlsInfo.ClientConfig()
		if errN != nil {
			return errN
		}
		EtcdDatabase, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{addresses},
			DialTimeout: 30 * time.Second,
			TLS: tlsConfig,
		})	
	}
	if err != nil {
		return err
	} else if EtcdDatabase == nil {
		return errors.New("could not initialize etcd")
	}
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
	ctx, _ := context.WithTimeout(context.Background(), 15 * time.Second)
	_, err = EtcdDatabase.Put(ctx, tableName, string(newTable))
//	
	if err != nil {
		return err
	}
	return nil
}

func etcdInsert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		ctx, _ := context.WithTimeout(context.Background(), 15 * time.Second)
		preDataList, err := EtcdDatabase.Get(ctx, tableName)
		
		if err != nil {
			return err
		}
		var preData []byte
		var preDataMap map[string]string
		if len(preDataList.Kvs) > 0 {
			preData = preDataList.Kvs[0].Value
			if err := json.Unmarshal(preData, &preDataMap); err != nil {
				return err
			}	
		} else {
			preDataMap = make(map[string]string)
		}
		preDataMap[key] = value
		postData, err := json.Marshal(&preDataMap)
		if err != nil {
			return err
		}
		_, err = EtcdDatabase.Put(ctx, tableName, string(postData))
		
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
		ctx, _ := context.WithTimeout(context.Background(), 15 * time.Second)
		preDataList, err := EtcdDatabase.Get(ctx, tableName)
		
		if err != nil {
			return err
		}
		var preData []byte
		var preDataMap map[string]string
		if len(preDataList.Kvs) > 0 {
			preData = preDataList.Kvs[0].Value
			if err := json.Unmarshal(preData, &preDataMap); err != nil {
				return err
			}	
		} else {
			preDataMap = make(map[string]string)
		}
		delete(preDataMap, key)
		postData, err := json.Marshal(&preDataMap)
		if err != nil {
			return err
		}
		_, err = EtcdDatabase.Put(ctx, tableName, string(postData))
		
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid delete, key is required")
	}
}

func etcdDeleteAllRecords(tableName string) error {
	ctx, _ := context.WithTimeout(context.Background(), 15 * time.Second)
	_, err := EtcdDatabase.Delete(ctx, tableName)
	
	if err != nil {
		return err
	}
	err = etcdCreateTable(tableName)
	if err != nil {
		return err
	}
	return nil
}

func etcdFetchRecords(tableName string) (map[string]string, error) {
	var records map[string]string
	ctx, _ := context.WithTimeout(context.Background(), 15 * time.Second)
	preDataList, err := EtcdDatabase.Get(ctx, tableName)
	
	if err != nil {
		return records, err
	}
	var preData []byte
	if len(preDataList.Kvs) > 0 {
		preData = preDataList.Kvs[0].Value
		if err = json.Unmarshal(preData, &records); err != nil {
			return nil, err
		}	
	} else {
		return nil, errors.New(NO_RECORDS)
	}
	return records, nil
}

func etcdCloseDB() {
	EtcdDatabase.Close()
}

func isValidIp(ipAddr string) bool {
	return net.ParseIP(ipAddr) == nil
}

func etcdPrintValues(preDataList clientv3.GetResponse) {
	log.Println("database returned " + string(len(preDataList.Kvs)) + "results")
	if servercfg.GetVerbose() > 1 {
		log.Println("results:")
		for _, ev := range preDataList.Kvs {
			log.Println("  ",ev.Key,ev.Value)
		}
	}
}
