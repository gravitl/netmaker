package pro

import (
	"crypto/rand"
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.org/x/crypto/nacl/box"
)

const (
	db_license_key = "netmaker-id-key-pair"
)

type apiServerConf struct {
	PrivateKey []byte `json:"private_key" binding:"required"`
	PublicKey  []byte `json:"public_key" binding:"required"`
}

// FetchApiServerKeys - fetches netmaker license keys for identification
// as well as secure communication with API
// if none present, it generates a new pair
func FetchApiServerKeys() (pub *[32]byte, priv *[32]byte, err error) {
	var returnData = apiServerConf{}
	currentData, err := database.FetchRecord(database.SERVERCONF_TABLE_NAME, db_license_key)
	if err != nil && !database.IsEmptyRecord(err) {
		return nil, nil, err
	} else if database.IsEmptyRecord(err) { // need to generate a new identifier pair
		pub, priv, err = box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		pubBytes, err := ncutils.ConvertKeyToBytes(pub)
		if err != nil {
			return nil, nil, err
		}
		privBytes, err := ncutils.ConvertKeyToBytes(priv)
		if err != nil {
			return nil, nil, err
		}
		returnData.PrivateKey = privBytes
		returnData.PublicKey = pubBytes
		record, err := json.Marshal(&returnData)
		if err != nil {
			return nil, nil, err
		}
		if err = database.Insert(db_license_key, string(record), database.SERVERCONF_TABLE_NAME); err != nil {
			return nil, nil, err
		}
	} else {
		if err = json.Unmarshal([]byte(currentData), &returnData); err != nil {
			return nil, nil, err
		}
		priv, err = ncutils.ConvertBytesToKey(returnData.PrivateKey)
		if err != nil {
			return nil, nil, err
		}
		pub, err = ncutils.ConvertBytesToKey(returnData.PublicKey)
		if err != nil {
			return nil, nil, err
		}
	}

	return pub, priv, nil
}
