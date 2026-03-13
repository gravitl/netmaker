package logic

type ServerSyncType string

const (
	SyncTypeSettings   ServerSyncType = "settings"
	SyncTypePeerUpdate ServerSyncType = "peerupdate"
	SyncTypeIDPSync    ServerSyncType = "idpsync"
)

// PublishServerSync is set by the mq package at startup to broadcast
// sync signals to peer servers in HA mode. The callback avoids a
// circular import (logic -> mq).
var PublishServerSync func(syncType ServerSyncType)
