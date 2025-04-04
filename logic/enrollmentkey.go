package logic

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slices"
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
	InvalidCreate:      fmt.Errorf("failed to create enrollment key. paramters invalid"),
	NoKeyFound:         fmt.Errorf("no enrollmentkey found"),
	InvalidKey:         fmt.Errorf("invalid key provided"),
	NoUsesRemaining:    fmt.Errorf("no uses remaining"),
	FailedToTokenize:   fmt.Errorf("failed to tokenize"),
	FailedToDeTokenize: fmt.Errorf("failed to detokenize"),
}
var (
	enrollmentkeyCacheMutex = &sync.RWMutex{}
	enrollmentkeyCacheMap   = make(map[string]models.EnrollmentKey)
)

// CreateEnrollmentKey - creates a new enrollment key in db
func CreateEnrollmentKey(uses int, expiration time.Time, networks, tags []string, groups []models.TagID, unlimited bool, relay uuid.UUID, defaultKey, autoEgress bool) (*models.EnrollmentKey, error) {
	newKeyID, err := getUniqueEnrollmentID()
	if err != nil {
		return nil, err
	}
	k := &models.EnrollmentKey{
		Value:         newKeyID,
		Expiration:    time.Time{},
		UsesRemaining: 0,
		Unlimited:     unlimited,
		Networks:      []string{},
		Tags:          []string{},
		Type:          models.Undefined,
		Relay:         relay,
		Groups:        groups,
		Default:       defaultKey,
		AutoEgress:    autoEgress,
	}
	if uses > 0 {
		k.UsesRemaining = uses
		k.Type = models.Uses
	} else if !expiration.IsZero() {
		k.Expiration = expiration
		k.Type = models.TimeExpiration
	} else if k.Unlimited {
		k.Type = models.Unlimited
	}
	if len(networks) > 0 {
		k.Networks = networks
	}
	if len(tags) > 0 {
		k.Tags = tags
	}
	if err := k.Validate(); err != nil {
		return nil, err
	}
	if relay != uuid.Nil {
		relayNode, err := GetNodeByID(relay.String())
		if err != nil {
			return nil, err
		}
		if !slices.Contains(k.Networks, relayNode.Network) {
			return nil, errors.New("relay node not in key's networks")
		}
		if !relayNode.IsRelay {
			return nil, errors.New("relay node is not a relay")
		}
	}
	if err = upsertEnrollmentKey(k); err != nil {
		return nil, err
	}
	return k, nil
}

// UpdateEnrollmentKey - updates an existing enrollment key's associated relay
func UpdateEnrollmentKey(keyId string, relayId uuid.UUID, groups []models.TagID, autoEgress bool) (*models.EnrollmentKey, error) {
	key, err := GetEnrollmentKey(keyId)
	if err != nil {
		return nil, err
	}

	if relayId != uuid.Nil {
		relayNode, err := GetNodeByID(relayId.String())
		if err != nil {
			return nil, err
		}
		if !slices.Contains(key.Networks, relayNode.Network) {
			return nil, errors.New("relay node not in key's networks")
		}
		if !relayNode.IsRelay {
			return nil, errors.New("relay node is not a relay")
		}
	}

	key.Relay = relayId
	key.Groups = groups
	key.AutoEgress = autoEgress
	if err = upsertEnrollmentKey(&key); err != nil {
		return nil, err
	}

	return &key, nil
}

// GetAllEnrollmentKeys - fetches all enrollment keys from DB
func GetAllEnrollmentKeys() ([]models.EnrollmentKey, error) {
	currentKeys, err := getEnrollmentKeysMap()
	if err != nil {
		return nil, err
	}
	var currentKeysList = []models.EnrollmentKey{}
	for k := range currentKeys {
		currentKeysList = append(currentKeysList, currentKeys[k])
	}
	return currentKeysList, nil
}

// GetEnrollmentKey - fetches a single enrollment key
// returns nil and error if not found
func GetEnrollmentKey(value string) (key models.EnrollmentKey, err error) {
	currentKeys, err := getEnrollmentKeysMap()
	if err != nil {
		return key, err
	}
	if key, ok := currentKeys[value]; ok {
		return key, nil
	}
	return key, EnrollmentErrors.NoKeyFound
}

func deleteEnrollmentkeyFromCache(key string) {
	enrollmentkeyCacheMutex.Lock()
	delete(enrollmentkeyCacheMap, key)
	enrollmentkeyCacheMutex.Unlock()
}

// DeleteEnrollmentKey - delete's a given enrollment key by value
func DeleteEnrollmentKey(value string, force bool) error {
	key, err := GetEnrollmentKey(value)
	if err != nil {
		return err
	}
	if key.Default && !force {
		return errors.New("cannot delete default network key")
	}
	err = database.DeleteRecord(database.ENROLLMENT_KEYS_TABLE_NAME, value)
	if err == nil {
		if servercfg.CacheEnabled() {
			deleteEnrollmentkeyFromCache(value)
		}
	}
	return err
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
	return &k, nil
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
	if err = upsertEnrollmentKey(&k); err != nil {
		return nil, err
	}

	return &k, nil
}

func upsertEnrollmentKey(k *models.EnrollmentKey) error {
	if k == nil {
		return EnrollmentErrors.InvalidKey
	}
	data, err := json.Marshal(k)
	if err != nil {
		return err
	}
	err = database.Insert(k.Value, string(data), database.ENROLLMENT_KEYS_TABLE_NAME)
	if err == nil {
		if servercfg.CacheEnabled() {
			storeEnrollmentkeyInCache(k.Value, *k)
		}
	}
	return nil
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

func getEnrollmentkeysFromCache() map[string]models.EnrollmentKey {
	return enrollmentkeyCacheMap
}

func storeEnrollmentkeyInCache(key string, enrollmentkey models.EnrollmentKey) {
	enrollmentkeyCacheMutex.Lock()
	enrollmentkeyCacheMap[key] = enrollmentkey
	enrollmentkeyCacheMutex.Unlock()
}

func getEnrollmentKeysMap() (map[string]models.EnrollmentKey, error) {
	if servercfg.CacheEnabled() {
		keys := getEnrollmentkeysFromCache()
		if len(keys) != 0 {
			return keys, nil
		}
	}
	records, err := database.FetchRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
	if err != nil {
		if !database.IsEmptyRecord(err) {
			return nil, err
		}
	}
	if records == nil {
		records = make(map[string]string)
	}
	currentKeys := make(map[string]models.EnrollmentKey, 0)
	if len(records) > 0 {
		for k := range records {
			var currentKey models.EnrollmentKey
			if err = json.Unmarshal([]byte(records[k]), &currentKey); err != nil {
				continue
			}
			currentKeys[k] = currentKey
			if servercfg.CacheEnabled() {
				storeEnrollmentkeyInCache(currentKey.Value, currentKey)
			}
		}
	}
	return currentKeys, nil
}

func RemoveTagFromEnrollmentKeys(deletedTagID models.TagID) {
	keys, _ := GetAllEnrollmentKeys()
	for _, key := range keys {
		newTags := []models.TagID{}
		update := false
		for _, tagID := range key.Groups {
			if tagID == deletedTagID {
				update = true
				continue
			}
			newTags = append(newTags, tagID)
		}
		if update {
			key.Groups = newTags
			upsertEnrollmentKey(&key)
		}

	}
}

func UnlinkNetworkAndTagsFromEnrollmentKeys(network string, delete bool) error {
	keys, err := GetAllEnrollmentKeys()
	if err != nil {
		return fmt.Errorf("failed to retrieve keys: %w", err)
	}

	var errs []error
	for _, key := range keys {
		newNetworks := []string{}
		newTags := []models.TagID{}
		update := false

		// Check and update networks
		for _, net := range key.Networks {
			if net == network {
				update = true
				continue
			}
			newNetworks = append(newNetworks, net)
		}

		// Check and update tags
		for _, tag := range key.Groups {
			tagParts := strings.Split(tag.String(), ".")
			if len(tagParts) == 0 {
				continue
			}
			tagNetwork := tagParts[0]
			if tagNetwork == network {
				update = true
				continue
			}
			newTags = append(newTags, tag)
		}

		if update && len(newNetworks) == 0 && delete {
			if err := DeleteEnrollmentKey(key.Value, true); err != nil {
				errs = append(errs, fmt.Errorf("failed to delete key %s: %w", key.Value, err))
			}
			continue
		}
		if update {
			key.Networks = newNetworks
			key.Groups = newTags
			if err := upsertEnrollmentKey(&key); err != nil {
				errs = append(errs, fmt.Errorf("failed to update key %s: %w", key.Value, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors unlinking network/tags from keys: %v", errs)
	}
	return nil
}
