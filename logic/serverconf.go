package logic

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

// GetJwtSecretValue fetches jwt secret from db
func GetJwtSecretValue() (string, error) {
	jwtSecret := &schema.Internal{
		Key: schema.InternalKey_JwtSecret,
	}
	err := jwtSecret.Get(db.WithContext(context.TODO()))
	if err != nil {
		return "", err
	}

	return jwtSecret.Value, nil
}

// StoreJWTSecret stores server jwt secret if needed
func StoreJWTSecret(privateKey string) error {
	jwtSecret := &schema.Internal{
		Key:   schema.InternalKey_JwtSecret,
		Value: privateKey,
	}
	ctx := db.WithContext(context.TODO())
	if jwtSecret.TenantID == "" {
		jwtSecret.TenantID = scope.ID(DefaultScope(ctx))
	}
	return jwtSecret.Set(ctx)
}
