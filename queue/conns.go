package queue

import "sync"

// ConnMap - map for holding http/ws connections and responses
var ConnMap sync.Map
