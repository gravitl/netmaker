package migrate

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

func migrateV1_5_3(ctx context.Context) error {
	return migrateServerConf(ctx)
}

func migrateServerConf(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable(database.SERVERCONF_TABLE_NAME) {
		return nil
	}

	records, err := kvList(ctx, database.SERVERCONF_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	record, ok := records["nm-jwt-secret"]
	if ok {
		recordData := make(map[string]string)
		err = json.Unmarshal([]byte(record), &recordData)
		if err != nil {
			return err
		}

		jwtSecretValue, ok := recordData["privatekey"]
		if ok {
			jwtSecret := &schema.Internal{
				Key:   schema.InternalKey_JwtSecret,
				Value: jwtSecretValue,
			}
			err = jwtSecret.Set(ctx)
			if err != nil {
				return err
			}
		}
	}

	record, ok = records["netmaker-id-key-pair"]
	if ok {
		recordData := make(map[string][]byte)
		err = json.Unmarshal([]byte(record), &recordData)
		if err != nil {
			return err
		}

		privateKeyValue, ok := recordData["private_key"]
		if ok {
			privateKey := &schema.Internal{
				Key:   schema.InternalKey_LicenseValidationPrivateKey,
				Value: base64.StdEncoding.EncodeToString(privateKeyValue),
			}
			err = privateKey.Set(ctx)
			if err != nil {
				return err
			}
		}

		publicKeyValue, ok := recordData["public_key"]
		if ok {
			publicKey := &schema.Internal{
				Key:   schema.InternalKey_LicenseValidationPublicKey,
				Value: base64.StdEncoding.EncodeToString(publicKeyValue),
			}
			err = publicKey.Set(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
