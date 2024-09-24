package logic

import (
	"encoding/json"
	"sort"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// InsertAcl - creates acl policy
func InsertAcl(a models.Acl) error {
	d, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return database.Insert(a.ID.String(), string(d), database.ACLS_TABLE_NAME)
}

func GetAcl(aID string) (models.Acl, error) {
	a := models.Acl{}
	d, err := database.FetchRecord(database.ACLS_TABLE_NAME, aID)
	if err != nil {
		return a, err
	}
	err = json.Unmarshal([]byte(d), &a)
	if err != nil {
		return a, err
	}
	return a, nil
}

// UpdateAcl - updates allowed fields on acls and commits to DB
func UpdateAcl(newAcl, acl models.Acl) error {
	if newAcl.Name != "" {
		acl.Name = newAcl.Name
	}
	acl.Src = newAcl.Src
	acl.Dst = newAcl.Dst
	acl.AllowedDirection = newAcl.AllowedDirection
	acl.Enabled = newAcl.Enabled
	d, err := json.Marshal(acl)
	if err != nil {
		return err
	}
	return database.Insert(acl.ID.String(), string(d), database.ACLS_TABLE_NAME)
}

// DeleteAcl - deletes acl policy
func DeleteAcl(a models.Acl) error {
	return database.DeleteRecord(database.ACLS_TABLE_NAME, a.ID.String())
}

// ListAcls - lists all acl policies
func ListAcls(netID models.NetworkID) ([]models.Acl, error) {
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
		if acl.NetworkID == netID {
			acls = append(acls, acl)
		}
	}
	return acls, nil
}

// SortTagEntrys - Sorts slice of Tag entries by their id
func SortAclEntrys(acls []models.Acl) {
	sort.Slice(acls, func(i, j int) bool {
		return acls[i].Name < acls[j].Name
	})
}
