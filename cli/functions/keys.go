package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// GetKeys - fetch all access keys of a network
func GetKeys(networkName string) *[]models.AccessKey {
	return request[[]models.AccessKey](http.MethodGet, fmt.Sprintf("/api/networks/%s/keys", networkName), nil)
}

// CreateKey - create an access key
func CreateKey(networkName string, key *models.AccessKey) *models.AccessKey {
	return request[models.AccessKey](http.MethodPost, fmt.Sprintf("/api/networks/%s/keys", networkName), key)
}

// DeleteKey - delete an access key
func DeleteKey(networkName, keyName string) {
	request[string](http.MethodDelete, fmt.Sprintf("/api/networks/%s/keys/%s", networkName, keyName), nil)
}
