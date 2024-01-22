package functions

import (
	"net/http"
	"net/url"

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

// DeleteEnrollmentKey - delete an enrollment key
func DeleteEnrollmentKey(keyID string) {
	request[any](http.MethodDelete, "/api/v1/enrollment-keys/"+url.QueryEscape(keyID), nil)
}
