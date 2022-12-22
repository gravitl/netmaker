package netcache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/database"
)

const (
	expirationTime = time.Minute * 5
)

// CValue - the cache object for a network
type CValue struct {
	Network    string    `json:"network"`
	Value      string    `json:"value"`
	Pass       string    `json:"pass"`
	User       string    `json:"user"`
	Expiration time.Time `json:"expiration"`
}

var ErrExpired = fmt.Errorf("expired")

// Set - sets a value to a key in db
func Set(k string, newValue *CValue) error {
	newValue.Expiration = time.Now().Add(expirationTime)
	newData, err := json.Marshal(newValue)
	if err != nil {
		return err
	}

	return database.Insert(k, string(newData), database.CACHE_TABLE_NAME)
}

// Get - gets a value from db, if expired, return err
func Get(k string) (*CValue, error) {
	record, err := database.FetchRecord(database.CACHE_TABLE_NAME, k)
	if err != nil {
		return nil, err
	}
	var entry CValue
	if err := json.Unmarshal([]byte(record), &entry); err != nil {
		return nil, err
	}
	if time.Now().After(entry.Expiration) {
		return nil, ErrExpired
	}

	return &entry, nil
}

// Del - deletes a value from db
func Del(k string) error {
	return database.DeleteRecord(database.CACHE_TABLE_NAME, k)
}
