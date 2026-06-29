package iru

import (
	"strings"
)

// deviceCompliant reports whether an Iru/Kandji device status (GET
// /api/v1/devices/{device_id}/status) satisfies the configured baseline.
// All parameters are evaluated; library_items are all checked unless
// compliance_library_item_ids limits evaluation to specific item IDs.
func deviceCompliant(status iruDeviceStatus, filterLibraryItemIDs map[string]struct{}) bool {
	if len(status.Parameters) == 0 && len(status.LibraryItems) == 0 {
		return true
	}
	for _, p := range status.Parameters {
		if !parameterStatusCompliant(p.Status) {
			return false
		}
	}
	checked := 0
	for _, item := range status.LibraryItems {
		if len(filterLibraryItemIDs) > 0 {
			if _, ok := filterLibraryItemIDs[item.ItemID]; !ok {
				continue
			}
		}
		checked++
		if !libraryItemStatusCompliant(item.Status) {
			return false
		}
	}
	if len(filterLibraryItemIDs) > 0 && checked == 0 && len(status.LibraryItems) > 0 {
		return false
	}
	return true
}

// parameterStatusCompliant maps Kandji parameter status values.
// See https://api-docs.kandji.io (Get Device Status).
func parameterStatusCompliant(status string) bool {
	switch normalizeIruStatus(status) {
	case "PASS", "REMEDIATED", "EXCLUDED", "WARNING":
		return true
	default:
		return false
	}
}

// libraryItemStatusCompliant maps Kandji library item status values.
func libraryItemStatusCompliant(status string) bool {
	switch normalizeIruStatus(status) {
	case "PASS", "SUCCESS", "EXCLUDED", "AVAILABLE":
		return true
	default:
		return false
	}
}

func normalizeIruStatus(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func libraryItemIDSet(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

type iruDeviceStatus struct {
	Parameters   []iruStatusItem `json:"parameters"`
	LibraryItems []iruStatusItem `json:"library_items"`
}

type iruStatusItem struct {
	ItemID string `json:"item_id"`
	Status string `json:"status"`
}
