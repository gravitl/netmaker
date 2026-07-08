package models

import "github.com/gravitl/netmaker/schema"

type Theme = schema.Theme

const (
	Dark   = schema.Dark
	Light  = schema.Light
	System = schema.System
)

type ServerSettings = schema.TenantSettings

type UserSettings struct {
	Theme         Theme  `json:"theme"`
	TextSize      string `json:"text_size"`
	ReducedMotion bool   `json:"reduced_motion"`
}
