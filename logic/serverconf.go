package logic

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

var (
	FreeTier = false
	// DefaultTrialEndDate - is a placeholder date for not applicable trial end dates
	DefaultTrialEndDate, _ = time.Parse("2006-Jan-02", "2021-Apr-01")

	GetTrialEndDate = func() (time.Time, error) {
		return DefaultTrialEndDate, nil
	}
)

const (
	__jwtSecret_internal_key = "jwt_secret"
)

type serverData struct {
	PrivateKey string `json:"privatekey,omitempty" bson:"privatekey,omitempty"`
}

// FetchJWTSecret - fetches jwt secret from db
func FetchJWTSecret() (string, error) {
	jwtSecret := &schema.Internal{
		Key: __jwtSecret_internal_key,
	}
	err := jwtSecret.Get(db.WithContext(context.TODO()))
	if err != nil {
		return "", err
	}

	return jwtSecret.Value, nil
}

// StoreJWTSecret - stores server jwt secret if needed
func StoreJWTSecret(privateKey string) error {
	jwtSecret := &schema.Internal{
		Key:   __jwtSecret_internal_key,
		Value: privateKey,
	}
	return jwtSecret.Set(db.WithContext(context.TODO()))
}
