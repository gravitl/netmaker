package models

import "time"

// DefaultExpDuration - the default expiration time of SsoState
const DefaultExpDuration = time.Minute * 5

// SsoState - holds SSO sign-in session data
type SsoState struct {
	Value      string    `json:"value"`
	Expiration time.Time `json:"expiration"`
}

// SsoState.IsExpired - tells if an SsoState is expired or not
func (s *SsoState) IsExpired() bool {
	return time.Now().After(s.Expiration)
}
