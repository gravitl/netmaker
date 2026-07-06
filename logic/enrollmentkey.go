package logic

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slices"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"context"
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

// CreateEnrollmentKey - creates a new enrollment key in db
func CreateEnrollmentKey(ctx context.Context, uses int, expiration time.Time, networks,
	tags []string, groups []models.TagID, unlimited bool, relay uuid.UUID,
	defaultKey, autoEgress, autoAssignGw bool) (*schema.EnrollmentKey, error) {

	newKeyID, err := getUniqueEnrollmentID(ctx)
	if err != nil {
		return nil, err
	}

	var keyType schema.EnrollmentKeyType
	var exp time.Time
	var usesRemaining int

	if uses > 0 {
		usesRemaining = uses
		keyType = schema.EnrollmentKeyType_LimitedUses
	} else if !expiration.IsZero() {
		exp = expiration
		keyType = schema.EnrollmentKeyType_TimedExpiry
	} else if unlimited {
		keyType = schema.EnrollmentKeyType_UnlimitedUses
	}

	keyTags := make(datatypes.JSONSlice[string], 0, len(groups))
	for _, g := range groups {
		keyTags = append(keyTags, g.String())
	}

	var relayPtr *string
	if relay != uuid.Nil {
		s := relay.String()
		relayPtr = &s
	}

	// tags[0] is the enrollment key display name
	name := ""
	if len(tags) > 0 {
		name = tags[0]
	}

	k := &schema.EnrollmentKey{
		ID:                uuid.NewString(),
		Name:              name,
		Value:             newKeyID,
		Expiration:        exp,
		UsesRemaining:     usesRemaining,
		Unlimited:         unlimited,
		Networks:          networks,
		Tags:              keyTags,
		Type:              keyType,
		GatewayID:         relayPtr,
		Default:           defaultKey,
		AutoEgress:        autoEgress,
		AutoAssignGateway: autoAssignGw,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if !enrollmentKeyIsValid(k) {
		return nil, fmt.Errorf("%w: uses remaining: %d, expiration: %s, unlimited: %t",
			models.ErrInvalidEnrollmentKey, k.UsesRemaining, k.Expiration, k.Unlimited)
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

	if defaultKey {
		if err := clearDefaultEnrollmentKeysForNetworks(ctx, networksForDefaultEnrollmentKey(k.Networks), ""); err != nil {
			return nil, err
		}
	}

	if k.TenantID == "" {
		k.TenantID = scope.ID(DefaultScope(ctx))
	}
	if err = k.Create(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

// CreateDefaultNetworkEnrollmentKey creates an unlimited default enrollment key for a network.
func CreateDefaultNetworkEnrollmentKey(networkName string) (*schema.EnrollmentKey, error) {
	ctx := db.WithContext(context.TODO())
	value, err := getUniqueEnrollmentID(ctx)
	if err != nil {
		return nil, err
	}

	key := &schema.EnrollmentKey{
		ID:        uuid.NewString(),
		Name:      networkName,
		Value:     value,
		Token:     "",
		Default:   true,
		Unlimited: true,
		Networks:  []string{networkName},
		Type:      schema.EnrollmentKeyType_UnlimitedUses,
	}
	if key.TenantID == "" {
		key.TenantID = scope.ID(DefaultScope(ctx))
	}
	err = key.Create(ctx)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// RegenerateEnrollmentKeyToken replaces the enrollment key value, invalidating any
// previously issued registration tokens while preserving key configuration.
func RegenerateEnrollmentKeyToken(ctx context.Context, keyValue string) (*schema.EnrollmentKey, error) {
	key := &schema.EnrollmentKey{Value: keyValue}
	if err := key.GetByValue(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, EnrollmentErrors.NoKeyFound
		}
		return nil, err
	}

	newValue, err := getUniqueEnrollmentID(ctx)
	if err != nil {
		return nil, err
	}

	key.Value = newValue
	key.Token = ""
	key.UpdatedAt = time.Now()

	if key.TenantID == "" {
		key.TenantID = scope.ID(DefaultScope(ctx))
	}
	if err := key.Upsert(ctx); err != nil {
		return nil, err
	}
	return key, nil
}

// UpdateEnrollmentKey - updates an existing enrollment key's relay and groups
func UpdateEnrollmentKey(ctx context.Context, keyValue string, updates *models.APIEnrollmentKey) (*schema.EnrollmentKey, error) {
	key := &schema.EnrollmentKey{Value: keyValue}
	if err := key.GetByValue(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, EnrollmentErrors.NoKeyFound
		}
		return nil, err
	}

	relayID := uuid.Nil
	if updates.Relay != "" {
		relayID = uuid.MustParse(updates.Relay)
	}

	if relayID != uuid.Nil {
		relayNode, err := GetNodeByID(relayID.String())
		if err != nil {
			return nil, err
		}
		if !slices.Contains(key.Networks, relayNode.Network) {
			return nil, errors.New("relay node not in key's networks")
		}
		if !relayNode.IsRelay {
			return nil, errors.New("relay node is not a relay")
		}
		updates.AutoAssignGateway = false
	}

	if relayID != uuid.Nil {
		s := relayID.String()
		key.GatewayID = &s
	} else {
		key.GatewayID = nil
	}

	keyTags := make(datatypes.JSONSlice[string], 0, len(updates.Groups))
	for _, g := range updates.Groups {
		keyTags = append(keyTags, g.String())
	}
	key.Tags = keyTags
	key.AutoAssignGateway = updates.AutoAssignGateway

	if !key.Default && updates.Default {
		if len(key.Tags) == 0 && len(key.Networks) == 0 {
			return nil, errors.New("default enrollment keys require at least one network or tag")
		}
		key.Default = true
		if err := clearDefaultEnrollmentKeysForNetworks(ctx, networksForDefaultEnrollmentKey(key.Networks), ""); err != nil {
			return nil, err
		}
	} else if key.Default && !updates.Default {
		key.Default = false
	}

	key.UpdatedAt = time.Now()
	if key.TenantID == "" {
		key.TenantID = scope.ID(DefaultScope(ctx))
	}
	if err := key.Upsert(ctx); err != nil {
		return nil, err
	}
	return key, nil
}

// GetAllEnrollmentKeys - fetches all enrollment keys from DB
func GetAllEnrollmentKeys(ctx context.Context) ([]schema.EnrollmentKey, error) {
	return (&schema.EnrollmentKey{}).ListAll(ctx)
}

// GetEnrollmentKey - fetches a single enrollment key by value
func GetEnrollmentKey(ctx context.Context, value string) (*schema.EnrollmentKey, error) {
	key := &schema.EnrollmentKey{Value: value}
	if err := key.GetByValue(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, EnrollmentErrors.NoKeyFound
		}
		return nil, err
	}
	return key, nil
}

// DeleteEnrollmentKey - deletes a given enrollment key by value
func DeleteEnrollmentKey(ctx context.Context, value string, force bool) error {
	key := &schema.EnrollmentKey{Value: value}
	if err := key.GetByValue(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnrollmentErrors.NoKeyFound
		}
		return err
	}
	if key.Default && !force {
		return errors.New("cannot delete default network key")
	}
	return key.DeleteByValue(ctx)
}

// GetDefaultEnrollmentKeyForNetwork returns the default enrollment key for a network.
func GetDefaultEnrollmentKeyForNetwork(ctx context.Context, network string) (*schema.EnrollmentKey, error) {
	keys, err := GetAllEnrollmentKeys(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Value < keys[j].Value })
	for i := range keys {
		if !keys[i].Default {
			continue
		}
		if enrollmentKeyAppliesToNetwork(keys[i], network) {
			return &keys[i], nil
		}
	}
	return nil, EnrollmentErrors.NoKeyFound
}

// TryToUseEnrollmentKey - checks first if key can be decremented
// returns true if it is decremented or isvalid
func TryToUseEnrollmentKey(ctx context.Context, k *schema.EnrollmentKey) bool {
	key, err := decrementEnrollmentKey(ctx, k.Value)
	if err != nil {
		if errors.Is(err, EnrollmentErrors.NoUsesRemaining) {
			return enrollmentKeyIsValid(k)
		}
	} else {
		k.UsesRemaining = key.UsesRemaining
		return true
	}
	return false
}

// Tokenize - tokenizes an enrollment key to be used via registration
// and attaches it to the Token field on the struct
func Tokenize(ctx context.Context, k *schema.EnrollmentKey, serverAddr string) error {
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
func DeTokenize(ctx context.Context, b64Token string) (*schema.EnrollmentKey, error) {
	if len(b64Token) == 0 {
		return nil, EnrollmentErrors.FailedToDeTokenize
	}
	tokenData, err := b64.StdEncoding.DecodeString(b64Token)
	if err != nil {
		return nil, err
	}

	var newToken models.EnrollmentToken
	if err = json.Unmarshal(tokenData, &newToken); err != nil {
		return nil, err
	}
	return GetEnrollmentKey(ctx, newToken.Value)
}

func RemoveTagFromEnrollmentKeys(deletedTagID models.TagID) {
	ctx := db.WithContext(context.TODO())
	keys, _ := GetAllEnrollmentKeys(ctx)
	for _, key := range keys {
		newTags := datatypes.JSONSlice[string]{}
		update := false
		for _, tagID := range key.Tags {
			if tagID == deletedTagID.String() {
				update = true
				continue
			}
			newTags = append(newTags, tagID)
		}
		if update {
			key.Tags = newTags
			key.UpdatedAt = time.Now()
			if key.TenantID == "" {
				key.TenantID = scope.ID(DefaultScope(ctx))
			}
			_ = key.Upsert(ctx)
		}
	}
}

func UnlinkNetworkAndTagsFromEnrollmentKeys(network string, delete bool) error {
	ctx := db.WithContext(context.TODO())
	keys, err := GetAllEnrollmentKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve keys: %w", err)
	}

	var errs []error
	for _, key := range keys {
		newNetworks := datatypes.JSONSlice[string]{}
		newTags := datatypes.JSONSlice[string]{}
		update := false

		for _, net := range key.Networks {
			if net == network {
				update = true
				continue
			}
			newNetworks = append(newNetworks, net)
		}

		for _, tag := range key.Tags {
			tagParts := strings.Split(tag, ".")
			if len(tagParts) == 0 {
				continue
			}
			if tagParts[0] == network {
				update = true
				continue
			}
			newTags = append(newTags, tag)
		}

		if update && len(newNetworks) == 0 && delete {
			if err := DeleteEnrollmentKey(ctx, key.Value, true); err != nil {
				errs = append(errs, fmt.Errorf("failed to delete key %s: %w", key.Value, err))
			}
			continue
		}
		if update {
			key.Networks = newNetworks
			key.Tags = newTags
			key.UpdatedAt = time.Now()
			if key.TenantID == "" {
				key.TenantID = scope.ID(DefaultScope(ctx))
			}
			if err := key.Upsert(ctx); err != nil {
				errs = append(errs, fmt.Errorf("failed to update key %s: %w", key.Value, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors unlinking network/tags from keys: %v", errs)
	}
	return nil
}

// == private ==

func enrollmentKeyIsValid(k *schema.EnrollmentKey) bool {
	if k == nil {
		return false
	}
	if k.UsesRemaining > 0 {
		return true
	}
	if !k.Expiration.IsZero() && time.Now().Before(k.Expiration) {
		return true
	}
	return k.Unlimited
}

func enrollmentKeyAppliesToNetwork(key schema.EnrollmentKey, network string) bool {
	return slices.Contains(key.Networks, network)
}

func networksForDefaultEnrollmentKey(networks datatypes.JSONSlice[string]) []string {
	seen := make(map[string]struct{}, len(networks))
	out := make([]string, 0, len(networks))
	for _, n := range networks {
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func clearDefaultEnrollmentKeysForNetworks(ctx context.Context, networks []string, exceptValue string) error {
	if len(networks) == 0 {
		return nil
	}
	keys, err := GetAllEnrollmentKeys(ctx)
	if err != nil {
		return err
	}
	networkSet := make(map[string]struct{}, len(networks))
	for _, n := range networks {
		networkSet[n] = struct{}{}
	}
	for i := range keys {
		if !keys[i].Default || keys[i].Value == exceptValue {
			continue
		}
		applies := false
		for network := range networkSet {
			if enrollmentKeyAppliesToNetwork(keys[i], network) {
				applies = true
				break
			}
		}
		if !applies {
			continue
		}
		keys[i].Default = false
		keys[i].UpdatedAt = time.Now()
		if keys[i].TenantID == "" {
			keys[i].TenantID = scope.ID(DefaultScope(ctx))
		}
		if err := keys[i].Upsert(ctx); err != nil {
			return err
		}
	}
	return nil
}

func decrementEnrollmentKey(ctx context.Context, value string) (*schema.EnrollmentKey, error) {
	k, err := GetEnrollmentKey(ctx, value)
	if err != nil {
		return nil, err
	}
	if k.UsesRemaining == 0 {
		return nil, EnrollmentErrors.NoUsesRemaining
	}
	k.UsesRemaining--
	k.UpdatedAt = time.Now()
	if k.TenantID == "" {
		k.TenantID = scope.ID(DefaultScope(ctx))
	}
	if err = k.Upsert(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

func getUniqueEnrollmentID(ctx context.Context) (string, error) {
	newID := RandomString(models.EnrollmentKeyLength)
	for {
		key := &schema.EnrollmentKey{Value: newID}
		err := key.GetByValue(ctx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newID, nil
		}
		if err != nil {
			return "", err
		}
		newID = RandomString(models.EnrollmentKeyLength)
	}
}
