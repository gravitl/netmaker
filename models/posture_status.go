package models

import (
	"time"

	"github.com/gravitl/netmaker/schema"
)

// HostPostureStatus is the netclient-facing summary of a host's last evaluated
// posture state. Returned by GET /api/v1/host/{hostid}/posture_status.
type HostPostureStatus struct {
	HostID      string                 `json:"host_id"`
	EvaluatedAt time.Time              `json:"evaluated_at"`
	MDM         *HostMDMStatus         `json:"mdm,omitempty"`
	EDR         *HostEDRStatus         `json:"edr,omitempty"`
	Networks    []NetworkPostureStatus `json:"networks"`
}

// HostEDRStatus is the current EDR sync snapshot for the host's configured
// EDR provider (if any).
type HostEDRStatus struct {
	Provider       string    `json:"provider"`
	MatchedBy      string    `json:"matched_by"`
	AgentInstalled bool      `json:"agent_installed"`
	AgentHealthy   bool      `json:"agent_healthy"`
	RiskLevel      string    `json:"risk_level"`
	LastSyncedAt   time.Time `json:"last_synced_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	LastError      string    `json:"last_error,omitempty"`
}

// HostMDMStatus is the current MDM sync snapshot for the host's configured
// MDM provider (if any).
type HostMDMStatus struct {
	Provider     string    `json:"provider"`
	MatchedBy    string    `json:"matched_by"`
	Enrolled     bool      `json:"enrolled"`
	Compliant    bool      `json:"compliant"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

// NetworkPostureStatus describes posture state for a single (host, network).
type NetworkPostureStatus struct {
	NetworkID  string          `json:"network_id"`
	NodeID     string          `json:"node_id"`
	Severity   schema.Severity `json:"severity"`
	Status     string          `json:"status"` // pass | warn | fail
	Violations []Violation     `json:"violations"`
}

// Network posture status values.
const (
	PostureStatusPass = "pass"
	PostureStatusWarn = "warn"
	PostureStatusFail = "fail"
)
