package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

var tagMutex = &sync.RWMutex{}

// GetTag - fetches tag info
func GetTag(tagID models.TagID) (models.Tag, error) {
	data, err := database.FetchRecord(database.TAG_TABLE_NAME, tagID.String())
	if err != nil {
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
	tagMutex.Lock()
	defer tagMutex.Unlock()
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
func DeleteTag(tagID models.TagID, removeFromPolicy bool) error {
	tagMutex.Lock()
	defer tagMutex.Unlock()
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
	if removeFromPolicy {
		// remove tag used on acl policy
		go RemoveDeviceTagFromAclPolicies(tagID, tag.Network)
	}
	extclients, _ := GetNetworkExtClients(tag.Network.String())
	for _, extclient := range extclients {
		if _, ok := extclient.Tags[tagID]; ok {
			delete(extclient.Tags, tagID)
			SaveExtClient(&extclient)
		}
	}
	return database.DeleteRecord(database.TAG_TABLE_NAME, tagID.String())
}

// ListTagsWithHosts - lists all tags with tagged hosts
func ListTagsWithNodes(netID models.NetworkID) ([]models.TagListResp, error) {
	tags, err := ListNetworkTags(netID)
	if err != nil {
		return []models.TagListResp{}, err
	}
	tagsNodeMap := GetTagMapWithNodesByNetwork(netID)
	resp := []models.TagListResp{}
	for _, tagI := range tags {
		tagRespI := models.TagListResp{
			Tag:         tagI,
			UsedByCnt:   len(tagsNodeMap[tagI.ID]),
			TaggedNodes: GetAllNodesAPI(tagsNodeMap[tagI.ID]),
		}
		resp = append(resp, tagRespI)
	}
	return resp, nil
}

// ListTags - lists all tags from DB
func ListTags() ([]models.Tag, error) {
	tagMutex.RLock()
	defer tagMutex.RUnlock()
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
	tagMutex.RLock()
	defer tagMutex.RUnlock()
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
	var err error
	tagNodesMap := GetNodesWithTag(req.ID)
	for _, apiNode := range req.TaggedNodes {
		node := models.Node{}
		var nodeID string
		if apiNode.IsStatic {
			if apiNode.StaticNode.RemoteAccessClientID != "" {
				continue
			}
			extclient, err := GetExtClient(apiNode.StaticNode.ClientID, apiNode.StaticNode.Network)
			if err != nil {
				continue
			}
			node.IsStatic = true
			nodeID = extclient.ClientID
			node.StaticNode = extclient
		} else {
			node, err = GetNodeByID(apiNode.ID)
			if err != nil {
				continue
			}
			nodeID = node.ID.String()
		}

		if _, ok := tagNodesMap[nodeID]; !ok {
			if node.StaticNode.Tags == nil {
				node.StaticNode.Tags = make(map[models.TagID]struct{})
			}
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			if newID != "" {
				if node.IsStatic {
					node.StaticNode.Tags[newID] = struct{}{}
					SaveExtClient(&node.StaticNode)
				} else {
					node.Tags[newID] = struct{}{}
					UpsertNode(&node)
				}

			} else {
				if node.IsStatic {
					node.StaticNode.Tags[req.ID] = struct{}{}
					SaveExtClient(&node.StaticNode)
				} else {
					node.Tags[req.ID] = struct{}{}
					UpsertNode(&node)
				}
			}
		} else {
			if newID != "" {
				delete(node.Tags, req.ID)
				delete(node.StaticNode.Tags, req.ID)
				if node.IsStatic {
					node.StaticNode.Tags[newID] = struct{}{}
					SaveExtClient(&node.StaticNode)
				} else {
					node.Tags[newID] = struct{}{}
					UpsertNode(&node)
				}
			}
			delete(tagNodesMap, nodeID)
		}

	}
	for _, deletedTaggedNode := range tagNodesMap {
		delete(deletedTaggedNode.Tags, req.ID)
		delete(deletedTaggedNode.StaticNode.Tags, req.ID)
		if deletedTaggedNode.IsStatic {
			SaveExtClient(&deletedTaggedNode.StaticNode)
		} else {
			UpsertNode(&deletedTaggedNode)
		}
	}
	go func(req models.UpdateTagReq) {
		if newID != "" {
			tagNodesMap = GetNodesWithTag(req.ID)
			for _, nodeI := range tagNodesMap {
				nodeI := nodeI
				if nodeI.StaticNode.Tags == nil {
					nodeI.StaticNode.Tags = make(map[models.TagID]struct{})
				}
				if nodeI.Tags == nil {
					nodeI.Tags = make(map[models.TagID]struct{})
				}
				delete(nodeI.Tags, req.ID)
				delete(nodeI.StaticNode.Tags, req.ID)
				nodeI.Tags[newID] = struct{}{}
				nodeI.StaticNode.Tags[newID] = struct{}{}
				if nodeI.IsStatic {
					SaveExtClient(&nodeI.StaticNode)
				} else {
					UpsertNode(&nodeI)
				}
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

func CheckIDSyntax(id string) error {
	if id == "" {
		return errors.New("name is required")
	}
	if len(id) < 3 {
		return errors.New("name should have min 3 characters")
	}
	reg, err := regexp.Compile("^[a-zA-Z-]+$")
	if err != nil {
		return err
	}
	if !reg.MatchString(id) {
		return errors.New("invalid name. allowed characters are [a-zA-Z-]")
	}
	return nil
}

func CreateDefaultTags(netID models.NetworkID) {
	// create tag for remote access gws in the network
	tag := models.Tag{
		ID:        models.TagID(fmt.Sprintf("%s.%s", netID.String(), models.RemoteAccessTagName)),
		TagName:   models.RemoteAccessTagName,
		Network:   netID,
		CreatedBy: "auto",
		CreatedAt: time.Now(),
	}
	_, err := GetTag(tag.ID)
	if err == nil {
		return
	}
	err = InsertTag(tag)
	if err != nil {
		slog.Error("failed to create remote access gw tag", "error", err.Error())
		return
	}
}
