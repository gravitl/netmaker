package models

import "github.com/gravitl/netmaker/schema"

// Theme enum constants live only in schema (avoids duplicate swagger enums).
type Theme = schema.Theme

type ServerSettings = schema.TenantSettings

type UserSettings struct {
	Theme         Theme  `json:"theme"`
	TextSize      string `json:"text_size"`
	ReducedMotion bool   `json:"reduced_motion"`
}
