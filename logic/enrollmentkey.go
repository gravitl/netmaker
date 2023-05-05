package logic

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// EnrollmentErrors - struct for holding EnrollmentKey error messages
var EnrollmentErrors = struct {
	InvalidCreate      error
	NoKeyFound         error
	InvalidKey         error
	NoUsesRemaining    error
	FailedToTokenize   error
	FailedToDeTokenize error
}{
	InvalidCreate:      fmt.Errorf("invalid enrollment key created"),
	NoKeyFound:         fmt.Errorf("no enrollmentkey found"),
	InvalidKey:         fmt.Errorf("invalid key provided"),
	NoUsesRemaining:    fmt.Errorf("no uses remaining"),
	FailedToTokenize:   fmt.Errorf("failed to tokenize"),
	FailedToDeTokenize: fmt.Errorf("failed to detokenize"),
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
		return nil, EnrollmentErrors.InvalidCreate
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
	var currentKeysList = []*models.EnrollmentKey{}
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
	return nil, EnrollmentErrors.NoKeyFound
}

// DeleteEnrollmentKey - delete's a given enrollment key by value
func DeleteEnrollmentKey(value string) error {
	_, err := GetEnrollmentKey(value)
	if err != nil {
		return err
	}
	return database.DeleteRecord(database.ENROLLMENT_KEYS_TABLE_NAME, value)
}

// TryToUseEnrollmentKey - checks first if key can be decremented
// returns true if it is decremented or isvalid
func TryToUseEnrollmentKey(k *models.EnrollmentKey) bool {
	key, err := decrementEnrollmentKey(k.Value)
	if err != nil {
		if errors.Is(err, EnrollmentErrors.NoUsesRemaining) {
			return k.IsValid()
		}
	} else {
		k.UsesRemaining = key.UsesRemaining
		return true
	}
	return false
}

// Tokenize - tokenizes an enrollment key to be used via registration
// and attaches it to the Token field on the struct
func Tokenize(k *models.EnrollmentKey, serverAddr string) error {
	if len(serverAddr) == 0 || k == nil {
		return EnrollmentErrors.FailedToTokenize
	}
	newToken := models.EnrollmentToken{
		Server: serverAddr,
		Value:  k.Value,
	}
	data, err := json.Marshal(&newToken)
	if err != nil {
		return err
	}
	k.Token = b64.StdEncoding.EncodeToString(data)
	return nil
}

// DeTokenize - detokenizes a base64 encoded string
// and finds the associated enrollment key
func DeTokenize(b64Token string) (*models.EnrollmentKey, error) {
	if len(b64Token) == 0 {
		return nil, EnrollmentErrors.FailedToDeTokenize
	}
	tokenData, err := b64.StdEncoding.DecodeString(b64Token)
	if err != nil {
		return nil, err
	}

	var newToken models.EnrollmentToken
	err = json.Unmarshal(tokenData, &newToken)
	if err != nil {
		return nil, err
	}
	k, err := GetEnrollmentKey(newToken.Value)
	if err != nil {
		return nil, err
	}
	return k, nil
}

// == private ==

// decrementEnrollmentKey - decrements the uses on a key if above 0 remaining
func decrementEnrollmentKey(value string) (*models.EnrollmentKey, error) {
	k, err := GetEnrollmentKey(value)
	if err != nil {
		return nil, err
	}
	if k.UsesRemaining == 0 {
		return nil, EnrollmentErrors.NoUsesRemaining
	}
	k.UsesRemaining = k.UsesRemaining - 1
	if err = upsertEnrollmentKey(k); err != nil {
		return nil, err
	}

	return k, nil
}

func upsertEnrollmentKey(k *models.EnrollmentKey) error {
	if k == nil {
		return EnrollmentErrors.InvalidKey
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
	newID := RandomString(models.EnrollmentKeyLength)
	for _, ok := currentKeys[newID]; ok; {
		newID = RandomString(models.EnrollmentKeyLength)
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
	currentKeys := make(map[string]*models.EnrollmentKey, 0)
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
