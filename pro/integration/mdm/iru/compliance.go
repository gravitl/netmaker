package iru

import (
	"strings"
)

// deviceCompliant reports whether an Iru device status satisfies the configured
// baseline. All parameters and library_items must have status PASS unless
// compliance_library_item_ids limits evaluation to specific library items.
func deviceCompliant(status iruDeviceStatus, filterLibraryItemIDs map[string]struct{}) bool {
	if len(status.Parameters) == 0 && len(status.LibraryItems) == 0 {
		return true
	}
	for _, p := range status.Parameters {
		if statusItemFailed(p.Status) {
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
		if statusItemFailed(item.Status) {
			return false
		}
	}
	if len(filterLibraryItemIDs) > 0 && checked == 0 && len(status.LibraryItems) > 0 {
		return false
	}
	return true
}

func statusItemFailed(status string) bool {
	return !strings.EqualFold(strings.TrimSpace(status), "PASS")
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
