package logic

import (
	"context"
	"encoding/base64"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

// RetrievePrivateTrafficKey retrieves private key of server
func RetrievePrivateTrafficKey() ([]byte, error) {
	mqPrivateKey := &schema.Internal{
		Key: schema.InternalKey_MqPrivateKey,
	}
	err := mqPrivateKey.Get(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(mqPrivateKey.Value)
}

// RetrievePublicTrafficKey retrieves public key of server
func RetrievePublicTrafficKey() ([]byte, error) {
	mqPublicKey := &schema.Internal{
		Key: schema.InternalKey_MqPublicKey,
	}
	err := mqPublicKey.Get(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(mqPublicKey.Value)
}
