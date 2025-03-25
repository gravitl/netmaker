package models

import (
	"fmt"
	"time"
)

// AccessToken - token used to access netmaker
type AccessToken struct {
	Name      string    `json:"name"`
	UserName  string    `json:"user_name"`
	ExpiresAt time.Time `json:"expires_at"`
	LastUsed  time.Time `json:"last_used"`
	CreatedBy time.Time `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (a AccessToken) GetDBKey() string {
	return fmt.Sprintf("%s#%s", a.UserName, a.Name)
}
