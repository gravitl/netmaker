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
	tag, err := GetTag(tagID)
	if err != nil {
		return err
	}
	nodes, err := GetNetworkNodes(tag.Network.String())
	if err != nil {
		return err
	}
	for _, nodeI := range nodes {
		nodeI := nodeI
		if _, ok := nodeI.Tags[tagID]; ok {
			delete(nodeI.Tags, tagID)
			UpsertNode(&nodeI)
		}
	}
	return database.DeleteRecord(database.TAG_TABLE_NAME, tagID.String())
}

// ListTagsWithHosts - lists all tags with tagged hosts
func ListTagsWithNodes(netID models.NetworkID) ([]models.TagListResp, error) {
	tagMutex.RLock()
	defer tagMutex.RUnlock()
	tags, err := ListNetworkTags(netID)
	if err != nil {
		return []models.TagListResp{}, err
	}
	tagsNodeMap := GetTagMapWithNodes(netID)
	resp := []models.TagListResp{}
	for _, tagI := range tags {
		tagRespI := models.TagListResp{
			Tag:         tagI,
			UsedByCnt:   len(tagsNodeMap[tagI.ID]),
			TaggedNodes: tagsNodeMap[tagI.ID],
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

// ListTags - lists all tags from DB
func ListNetworkTags(netID models.NetworkID) ([]models.Tag, error) {

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
		if tag.Network == netID {
			tags = append(tags, tag)
		}

	}
	return tags, nil
}

// UpdateTag - updates and syncs hosts with tag update
func UpdateTag(req models.UpdateTagReq, newID models.TagID) {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	tagNodesMap := GetNodesWithTag(req.ID)
	for _, nodeID := range req.TaggedNodes {
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}

		if _, ok := tagNodesMap[node.ID.String()]; !ok {
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			node.Tags[req.ID] = struct{}{}
			UpsertNode(&node)
		} else {
			delete(tagNodesMap, node.ID.String())
		}
	}
	for _, deletedTaggedNode := range tagNodesMap {
		deletedTaggedHost := deletedTaggedNode
		delete(deletedTaggedHost.Tags, req.ID)
		UpsertNode(&deletedTaggedHost)
	}
	go func(req models.UpdateTagReq) {
		if newID != "" {
			tagNodesMap = GetNodesWithTag(req.ID)
			for _, nodeI := range tagNodesMap {
				nodeI := nodeI
				delete(nodeI.Tags, req.ID)
				nodeI.Tags[newID] = struct{}{}
				UpsertNode(&nodeI)
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