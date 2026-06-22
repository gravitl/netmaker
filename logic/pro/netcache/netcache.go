package netcache

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

const (
	expirationTime = time.Minute * 5
)

var ErrExpired = fmt.Errorf("expired")

// Set - sets a value to a key in db
func Set(k string, newValue *schema.CacheValue) error {
	newValue.Expiration = time.Now().Add(expirationTime)
	entry := &schema.CacheEntry{
		Key:   k,
		Value: datatypes.NewJSONType(*newValue),
	}
	return entry.Save(db.WithContext(context.TODO()))
}

// Get - gets a value from db, if expired, return err
func Get(k string) (*schema.CacheValue, error) {
	entry := &schema.CacheEntry{Key: k}
	if err := entry.Get(db.WithContext(context.TODO())); err != nil {
		return nil, err
	}
	v := entry.Value.Data()
	if time.Now().After(v.Expiration) {
		return nil, ErrExpired
	}
	return &v, nil
}

// Del - deletes a value from db
func Del(k string) error {
	return (&schema.CacheEntry{Key: k}).Delete(db.WithContext(context.TODO()))
}
