package database

import "strings"

// IsEmptyRecord - checks for if it's an empty record error or not
func IsEmptyRecord(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), NO_RECORD) || strings.Contains(err.Error(), NO_RECORDS)
}
