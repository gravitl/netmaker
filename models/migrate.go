package models

// MigrationData struct needed to create new v0.18.0 node from v.0.17.X node
type MigrationData struct {
	HostName    string
	Password    string
	OS          string
	LegacyNodes []LegacyNode
}
