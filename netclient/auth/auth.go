package auth

import (
	"os"

	"github.com/gravitl/netmaker/netclient/ncutils"
	//    "os"
)

// StoreSecret - stores auth secret locally
func StoreSecret(key string, network string) error {
	d1 := []byte(key)
	return os.WriteFile(ncutils.GetNetclientPathSpecific()+"secret-"+network, d1, 0600)
}

// RetrieveSecret - fetches secret locally
func RetrieveSecret(network string) (string, error) {
	dat, err := ncutils.GetFileWithRetry(ncutils.GetNetclientPathSpecific()+"secret-"+network, 3)
	return string(dat), err
}

// StoreTrafficKey - stores traffic key
func StoreTrafficKey(key *[32]byte, network string) error {
	var data, err = ncutils.ConvertKeyToBytes(key)
	if err != nil {
		return err
	}
	return os.WriteFile(ncutils.GetNetclientPathSpecific()+"traffic-"+network, data, 0600)
}

// RetrieveTrafficKey - reads traffic file locally
func RetrieveTrafficKey(network string) (*[32]byte, error) {
	data, err := ncutils.GetFileWithRetry(ncutils.GetNetclientPathSpecific()+"traffic-"+network, 2)
	if err != nil {
		return nil, err
	}
	return ncutils.ConvertBytesToKey(data)
}

// Configuraion - struct for mac and pass
type Configuration struct {
	MacAddress string
	Password   string
}
