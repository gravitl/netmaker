package pro

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models/promodels"
)

// InitializeGroups - initialize groups data structure if not present in the DB
func InitializeGroups() error {
	if !DoesUserGroupExist(DEFAULT_ALLOWED_GROUPS) {
		return InsertUserGroup(DEFAULT_ALLOWED_GROUPS)
	}
	return nil
}

// InsertUserGroup - inserts a group into the
func InsertUserGroup(groupName promodels.UserGroupName) error {
	currentGroups, err := GetUserGroups()
	if err != nil {
		return err
	}
	currentGroups[groupName] = promodels.Void{}
	newData, err := json.Marshal(&currentGroups)
	if err != nil {
		return err
	}
	return database.Insert(DB_GROUPS_KEY, string(newData), database.USER_GROUPS_TABLE_NAME)
}

// DeleteUserGroup - deletes a group from database
func DeleteUserGroup(groupName promodels.UserGroupName) error {
	var newGroups promodels.UserGroups
	currentGroupRecords, err := database.FetchRecord(database.USER_GROUPS_TABLE_NAME, DB_GROUPS_KEY)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	if err = json.Unmarshal([]byte(currentGroupRecords), &newGroups); err != nil {
		return err
	}
	delete(newGroups, groupName)
	newData, err := json.Marshal(&newGroups)
	if err != nil {
		return err
	}
	return database.Insert(DB_GROUPS_KEY, string(newData), database.USER_GROUPS_TABLE_NAME)
}

// GetUserGroups - get groups of users
func GetUserGroups() (promodels.UserGroups, error) {
	var returnGroups promodels.UserGroups
	groupsRecord, err := database.FetchRecord(database.USER_GROUPS_TABLE_NAME, DB_GROUPS_KEY)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return make(promodels.UserGroups, 1), nil
		}
		return returnGroups, err
	}

	if err = json.Unmarshal([]byte(groupsRecord), &returnGroups); err != nil {
		return returnGroups, err
	}

	return returnGroups, nil
}

// DoesUserGroupExist - checks if a user group exists
func DoesUserGroupExist(group promodels.UserGroupName) bool {
	currentGroups, err := GetUserGroups()
	if err != nil {
		return true
	}
	for k := range currentGroups {
		if k == group {
			return true
		}
	}
	return false
}
