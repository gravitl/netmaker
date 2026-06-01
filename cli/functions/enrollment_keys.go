package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// CreateEnrollmentKey - create an enrollment key
func CreateEnrollmentKey(key *models.APIEnrollmentKey) *models.EnrollmentKey {
	return request[models.EnrollmentKey](http.MethodPost, "/api/v1/enrollment-keys", key)
}

// GetEnrollmentKeys - gets all enrollment keys
func GetEnrollmentKeys() *[]models.EnrollmentKey {
	return request[[]models.EnrollmentKey](http.MethodGet, "/api/v1/enrollment-keys", nil)
}

// GetDefaultEnrollmentKeyForNetwork - gets the default enrollment key for a network
func GetDefaultEnrollmentKeyForNetwork(network string) *models.EnrollmentKey {
	return request[models.EnrollmentKey](http.MethodGet, "/api/v1/enrollment-keys/network/"+network+"/default", nil)
}

// DeleteEnrollmentKey - delete an enrollment key
func DeleteEnrollmentKey(keyID string) {
	request[any](http.MethodDelete, "/api/v1/enrollment-keys/"+keyID, nil)
}

// RegenerateEnrollmentKeyToken - regenerates an enrollment key token
func RegenerateEnrollmentKeyToken(keyID string) *models.EnrollmentKey {
	return request[models.EnrollmentKey](http.MethodPost, "/api/v1/enrollment-keys/"+keyID+"/regenerate-token", nil)
}
