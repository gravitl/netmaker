package logic

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// EnrollmentKeyErrors - struct for holding EnrollmentKey error messages
var EnrollmentKeyErrors = struct {
	InvalidCreate   string
	NoKeyFound      string
	InvalidKey      string
	NoUsesRemaining string
}{
	InvalidCreate:   "invalid enrollment key created",
	NoKeyFound:      "no enrollmentkey found",
	InvalidKey:      "invalid key provided",
	NoUsesRemaining: "no uses remaining",
}

// CreateEnrollmentKey - creates a new enrollment key in db
func CreateEnrollmentKey(uses int, expiration time.Time, networks, tags []string, unlimited bool) (k *models.EnrollmentKey, err error) {
	newKeyID, err := getUniqueEnrollmentID()
	if err != nil {
		return nil, err
	}
	k = &models.EnrollmentKey{
		Value:         newKeyID,
		Expiration:    time.Time{},
		UsesRemaining: 0,
		Unlimited:     unlimited,
		Networks:      []string{},
		Tags:          []string{},
	}
	if uses > 0 {
		k.UsesRemaining = uses
	}
	if !expiration.IsZero() {
		k.Expiration = expiration
	}
	if len(networks) > 0 {
		k.Networks = networks
	}
	if len(tags) > 0 {
		k.Tags = tags
	}
	if ok := k.Validate(); !ok {
		return nil, fmt.Errorf(EnrollmentKeyErrors.InvalidCreate)
	}
	if err = upsertEnrollmentKey(k); err != nil {
		return nil, err
	}
	return
}

// GetAllEnrollmentKeys - fetches all enrollment keys from DB
func GetAllEnrollmentKeys() ([]*models.EnrollmentKey, error) {
	currentKeys, err := getEnrollmentKeysMap()
	if err != nil {
		return nil, err
	}
	var currentKeysList = make([]*models.EnrollmentKey, len(currentKeys))
	for k := range currentKeys {
		currentKeysList = append(currentKeysList, currentKeys[k])
	}
	return currentKeysList, nil
}

// GetEnrollmentKey - fetches a single enrollment key
// returns nil and error if not found
func GetEnrollmentKey(value string) (*models.EnrollmentKey, error) {
	currentKeys, err := getEnrollmentKeysMap()
	if err != nil {
		return nil, err
	}
	if key, ok := currentKeys[value]; ok {
		return key, nil
	}
	return nil, fmt.Errorf(EnrollmentKeyErrors.NoKeyFound)
}

// DeleteEnrollmentKey - delete's a given enrollment key by value
func DeleteEnrollmentKey(value string) error {
	_, err := GetEnrollmentKey(value)
	if err != nil {
		return err
	}
	return database.DeleteRecord(database.ENROLLMENT_KEYS_TABLE_NAME, value)
}

// DecrementEnrollmentKey - decrements the uses on a key if above 0 remaining
func DecrementEnrollmentKey(value string) (*models.EnrollmentKey, error) {
	k, err := GetEnrollmentKey(value)
	if err != nil {
		return nil, err
	}
	if k.UsesRemaining == 0 {
		return nil, fmt.Errorf(EnrollmentKeyErrors.NoUsesRemaining)
	}
	k.UsesRemaining = k.UsesRemaining - 1
	if err = upsertEnrollmentKey(k); err != nil {
		return nil, err
	}

	return k, nil
}

// == private ==

func upsertEnrollmentKey(k *models.EnrollmentKey) error {
	if k == nil {
		return fmt.Errorf(EnrollmentKeyErrors.InvalidKey)
	}
	data, err := json.Marshal(k)
	if err != nil {
		return err
	}
	return database.Insert(k.Value, string(data), database.ENROLLMENT_KEYS_TABLE_NAME)
}

func getUniqueEnrollmentID() (string, error) {
	currentKeys, err := getEnrollmentKeysMap()
	if err != nil {
		return "", err
	}
	newID := ncutils.MakeRandomString(32)
	for _, ok := currentKeys[newID]; ok; {
		newID = ncutils.MakeRandomString(32)
	}
	return newID, nil
}

func getEnrollmentKeysMap() (map[string]*models.EnrollmentKey, error) {
	records, err := database.FetchRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
	if err != nil {
		if !database.IsEmptyRecord(err) {
			return nil, err
		}
	}
	if records == nil {
		records = make(map[string]string)
	}
	currentKeys := make(map[string]*models.EnrollmentKey)
	if len(records) > 0 {
		for k := range records {
			var currentKey models.EnrollmentKey
			if err = json.Unmarshal([]byte(records[k]), &currentKey); err != nil {
				continue
			}
			currentKeys[k] = &currentKey
		}
	}
	return currentKeys, nil
}
