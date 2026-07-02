package netcache

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const (
	expirationTime = time.Minute * 5
)

// CValue is the cache object for a network.
type CValue = schema.CacheValue

var ErrExpired = fmt.Errorf("expired")

// Set - sets a value to a key in db
func Set(k string, newValue *CValue) error {
	newValue.Expiration = time.Now().Add(expirationTime)
	r := &schema.CacheRecord{Key: k, Value: datatypes.NewJSONType(*newValue)}
	ctx := db.WithContext(context.TODO())
	if r.TenantID == "" {
		r.TenantID = scope.ID(logic.DefaultScope(ctx))
	}
	return r.Upsert(ctx)
}

// Get - gets a value from db, if expired, return err
func Get(k string) (*CValue, error) {
	r := &schema.CacheRecord{Key: k}
	if err := r.Get(db.WithContext(context.TODO())); err != nil {
		return nil, err
	}
	entry := r.Value.Data()
	if time.Now().After(entry.Expiration) {
		return nil, ErrExpired
	}
	return &entry, nil
}

// Del - deletes a value from db
func Del(k string) error {
	return (&schema.CacheRecord{Key: k}).Delete(db.WithContext(context.TODO()))
}
