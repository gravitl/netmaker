package models

import "github.com/gravitl/netmaker/schema"

type UserSettings struct {
	Theme         schema.Theme `json:"theme"`
	TextSize      string       `json:"text_size"`
	ReducedMotion bool         `json:"reduced_motion"`
}
