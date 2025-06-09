package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	Undefined KeyType = iota
	TimeExpiration
	Uses
	Unlimited
)

var (
	ErrNilEnrollmentKey          = errors.New("enrollment key is nil")
	ErrNilNetworksEnrollmentKey  = errors.New("enrollment key networks is nil")
	ErrNilTagsEnrollmentKey      = errors.New("enrollment key tags is nil")
	ErrInvalidEnrollmentKey      = errors.New("enrollment key is not valid")
	ErrInvalidEnrollmentKeyValue = errors.New("enrollment key value is not valid")
)

// KeyType - the type of enrollment key
type KeyType int

// String - returns the string representation of a KeyType
func (k KeyType) String() string {
	return [...]string{"Undefined", "TimeExpiration", "Uses", "Unlimited"}[k]
}

// EnrollmentToken - the tokenized version of an enrollmentkey;
// to be used for host registration
type EnrollmentToken struct {
	Server string `json:"server"`
	Value  string `json:"value"`
}

// EnrollmentKeyLength - the length of an enrollment key - 62^16 unique possibilities
const EnrollmentKeyLength = 32

// EnrollmentKey - the key used to register hosts and join them to specific networks
type EnrollmentKey struct {
	Expiration    time.Time `json:"expiration"`
	UsesRemaining int       `json:"uses_remaining"`
	Value         string    `json:"value"`
	Networks      []string  `json:"networks"`
	Unlimited     bool      `json:"unlimited"`
	Tags          []string  `json:"tags"`
	Token         string    `json:"token,omitempty"` // B64 value of EnrollmentToken
	Type          KeyType   `json:"type"`
	Relay         uuid.UUID `json:"relay"`
	Groups        []TagID   `json:"groups"`
	Default       bool      `json:"default"`
	AutoEgress    bool      `json:"auto_egress"`
}

// APIEnrollmentKey - used to create enrollment keys via API
type APIEnrollmentKey struct {
	Expiration    int64    `json:"expiration" swaggertype:"primitive,integer" format:"int64"`
	UsesRemaining int      `json:"uses_remaining"`
	Networks      []string `json:"networks"`
	Unlimited     bool     `json:"unlimited"`
	Tags          []string `json:"tags" validate:"required,dive,min=3,max=32"`
	Type          KeyType  `json:"type"`
	Relay         string   `json:"relay"`
	Groups        []TagID  `json:"groups"`
	AutoEgress    bool     `json:"auto_egress"`
}

// RegisterResponse - the response to a successful enrollment register
type RegisterResponse struct {
	ServerConf    ServerConfig `json:"server_config"`
	RequestedHost Host         `json:"requested_host"`
}

// EnrollmentKey.IsValid - checks if the key is still valid to use
func (k *EnrollmentKey) IsValid() bool {
	if k == nil {
		return false
	}
	if k.UsesRemaining > 0 {
		return true
	}
	if !k.Expiration.IsZero() && time.Now().Before(k.Expiration) {
		return true
	}
	if k.Type == Undefined {
		return false
	}

	return k.Unlimited
}

// EnrollmentKey.Validate - validate's an EnrollmentKey
// should be used during creation
func (k *EnrollmentKey) Validate() error {
	if k == nil {
		return ErrNilEnrollmentKey
	}
	if k.Tags == nil {
		return ErrNilTagsEnrollmentKey
	}
	if len(k.Value) != EnrollmentKeyLength {
		return fmt.Errorf("%w: length not %d characters", ErrInvalidEnrollmentKeyValue, EnrollmentKeyLength)
	}
	if !k.IsValid() {
		return fmt.Errorf("%w: uses remaining: %d, expiration: %s, unlimited: %t", ErrInvalidEnrollmentKey, k.UsesRemaining, k.Expiration, k.Unlimited)
	}
	return nil
}
