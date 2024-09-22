package logic

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

var tagMutex = &sync.RWMutex{}

// GetTag - fetches tag info
func GetTag(tagID models.TagID) (models.Tag, error) {
	data, err := database.FetchRecord(database.TAG_TABLE_NAME, tagID.String())
	if err != nil && !database.IsEmptyRecord(err) {
		return models.Tag{}, err
	}
	tag := models.Tag{}
	err = json.Unmarshal([]byte(data), &tag)
	if err != nil {
		return tag, err
	}
	return tag, nil
}

// InsertTag - creates new tag
func InsertTag(tag models.Tag) error {
	_, err := database.FetchRecord(database.TAG_TABLE_NAME, tag.ID.String())
	if err == nil {
		return fmt.Errorf("tag `%s` exists already", tag.ID)
	}
	d, err := json.Marshal(tag)
	if err != nil {
		return err
	}
	return database.Insert(tag.ID.String(), string(d), database.TAG_TABLE_NAME)
}

// DeleteTag - delete tag, will also untag hosts
func DeleteTag(tagID models.TagID) error {
	// cleanUp tags on hosts
	hosts, err := GetAllHosts()
	if err != nil {
		return err
	}
	for _, hostI := range hosts {
		hostI := hostI
		if _, ok := hostI.Tags[tagID]; ok {
			delete(hostI.Tags, tagID)
			UpsertHost(&hostI)
		}
	}
	return database.DeleteRecord(database.TAG_TABLE_NAME, tagID.String())
}

// ListTagsWithHosts - lists all tags with tagged hosts
func ListTagsWithHosts() ([]models.TagListResp, error) {
	tagMutex.RLock()
	defer tagMutex.RUnlock()
	tags, err := ListTags()
	if err != nil {
		return []models.TagListResp{}, err
	}
	tagsHostMap := GetTagMapWithHosts()
	resp := []models.TagListResp{}
	for _, tagI := range tags {
		tagRespI := models.TagListResp{
			Tag:         tagI,
			UsedByCnt:   len(tagsHostMap[tagI.ID]),
			TaggedHosts: tagsHostMap[tagI.ID],
		}
		resp = append(resp, tagRespI)
	}
	return resp, nil
}

// ListTags - lists all tags from DB
func ListTags() ([]models.Tag, error) {

	data, err := database.FetchRecords(database.TAG_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Tag{}, err
	}
	tags := []models.Tag{}
	for _, dataI := range data {
		tag := models.Tag{}
		err := json.Unmarshal([]byte(dataI), &tag)
		if err != nil {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// UpdateTag - updates and syncs hosts with tag update
func UpdateTag(req models.UpdateTagReq, newID models.TagID) {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	tagHostsMap := GetHostsWithTag(req.ID)
	for _, hostID := range req.TaggedHosts {
		hostI, err := GetHost(hostID)
		if err != nil {
			continue
		}
		if _, ok := tagHostsMap[hostI.ID.String()]; !ok {
			if hostI.Tags == nil {
				hostI.Tags = make(map[models.TagID]struct{})
			}
			hostI.Tags[req.ID] = struct{}{}
			UpsertHost(hostI)
		} else {
			delete(tagHostsMap, hostI.ID.String())
		}
	}
	for _, deletedTaggedHost := range tagHostsMap {
		deletedTaggedHost := deletedTaggedHost
		delete(deletedTaggedHost.Tags, req.ID)
		UpsertHost(&deletedTaggedHost)
	}
	go func(req models.UpdateTagReq) {
		if newID != "" {
			tagHostsMap = GetHostsWithTag(req.ID)
			for _, hostI := range tagHostsMap {
				hostI := hostI
				delete(hostI.Tags, req.ID)
				hostI.Tags[newID] = struct{}{}
				UpsertHost(&hostI)
			}
		}
	}(req)

}

// SortTagEntrys - Sorts slice of Tag entries by their id
func SortTagEntrys(tags []models.TagListResp) {
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].ID < tags[j].ID
	})
}
