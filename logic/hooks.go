package logic

// OnCacheInvalidation is set by the mq package at startup to broadcast
// cache invalidation signals to peer servers in HA mode. The callback
// avoids a circular import (logic -> mq). cacheType identifies the
// cache ("settings", "peerupdate") and key is an optional scope
// (empty = invalidate all).
var OnCacheInvalidation func(cacheType string, key string)
