package models

import "github.com/gravitl/netmaker/schema"

type Subject struct {
	ID   string             `json:"id"`
	Name string             `json:"name"`
	Type schema.SubjectType `json:"subject_type"`
	Info interface{}        `json:"info"`
}

type Diff struct {
	Old interface{}
	New interface{}
}

type Event struct {
	Action      schema.Action
	Source      Subject
	Origin      schema.Origin
	Target      Subject
	TriggeredBy string
	NetworkID   schema.NetworkID
	Diff        Diff
}
