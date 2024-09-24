package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// Create - creates acl policy
func Create(a models.Acl) error {
	d, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return database.Insert(a.ID.String(), string(d), database.ACLS_TABLE_NAME)
}

// Delete - deletes acl policy
func Delete(a models.Acl) error {
	return database.DeleteRecord(database.ACLS_TABLE_NAME, a.ID.String())
}

// List - lists all acl policies
func List(a models.Acl) ([]models.Acl, error) {
	data, err := database.FetchRecords(database.TAG_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}, err
	}
	acls := []models.Acl{}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}
		acls = append(acls, acl)
	}
	return acls, nil
}
