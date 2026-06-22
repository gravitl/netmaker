package logic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
)

var tagMutex = &sync.RWMutex{}

// GetTag - fetches tag info
func GetTag(tagID schema.TagID) (schema.Tag, error) {
	entry := &schema.TagEntry{Key: tagID.String()}
	if err := entry.Get(db.WithContext(context.TODO())); err != nil {
		return schema.Tag{}, err
	}
	return entry.Value.Data(), nil
}

func UpsertTag(tag schema.Tag) error {
	return (&schema.TagEntry{Key: tag.ID.String(), NetworkID: string(tag.Network), Value: datatypes.NewJSONType(tag)}).Save(db.WithContext(context.TODO()))
}

// InsertTag - creates new tag
func InsertTag(tag schema.Tag) error {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	existing := &schema.TagEntry{Key: tag.ID.String()}
	if err := existing.Get(db.WithContext(context.TODO())); err == nil {
		return fmt.Errorf("tag `%s` exists already", tag.ID)
	}
	return (&schema.TagEntry{Key: tag.ID.String(), NetworkID: string(tag.Network), Value: datatypes.NewJSONType(tag)}).Save(db.WithContext(context.TODO()))
}

// DeleteTag - delete tag, will also untag hosts
func DeleteTag(tagID schema.TagID, removeFromPolicy bool) error {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	// cleanUp tags on hosts
	tag, err := GetTag(tagID)
	if err != nil {
		return err
	}
	network := &schema.Network{
		Name: tag.Network.String(),
	}
	err = network.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	_ = (&schema.Node{}).UnassignTag(
		db.WithContext(context.TODO()),
		tag.ID.String(),
		dbtypes.WithFilter("network_id", network.ID),
	)

	if removeFromPolicy {
		// remove tag used on acl policy
		go RemoveDeviceTagFromAclPolicies(tagID, tag.Network)
	}
	go RemoveTagFromEgress(tag.Network, tagID)
	extclients, _ := logic.GetNetworkExtClients(tag.Network.String())
	for _, extclient := range extclients {
		if _, ok := extclient.Tags[tagID]; ok {
			delete(extclient.Tags, tagID)
			logic.SaveExtClient(&extclient)
		}
	}
	return (&schema.TagEntry{Key: tagID.String()}).Delete(db.WithContext(context.TODO()))
}

// ListTagsWithHosts - lists all tags with tagged hosts
func ListTagsWithNodes(netID schema.NetworkID) ([]models.TagListResp, error) {
	tags, err := ListNetworkTags(netID)
	if err != nil {
		return []models.TagListResp{}, err
	}
	tagsNodeMap := GetTagMapWithNodesByNetwork(netID, true)
	resp := []models.TagListResp{}
	for _, tagI := range tags {
		tagRespI := models.TagListResp{
			Tag:         tagI,
			UsedByCnt:   len(tagsNodeMap[tagI.ID]),
			TaggedNodes: logic.GetAllNodesAPI(tagsNodeMap[tagI.ID]),
		}
		resp = append(resp, tagRespI)
	}
	return resp, nil
}
func DeleteAllNetworkTags(networkID schema.NetworkID) {
	tags, _ := ListNetworkTags(networkID)
	for _, tagI := range tags {
		DeleteTag(tagI.ID, false)
	}
}

// ListNetworkTags - lists all tags in network
func ListNetworkTags(netID schema.NetworkID) ([]schema.Tag, error) {
	tagMutex.RLock()
	defer tagMutex.RUnlock()
	entries, err := (&schema.TagEntry{}).ListAll(db.WithContext(context.TODO()))
	if err != nil && !database.IsEmptyRecord(err) {
		return []schema.Tag{}, err
	}
	tags := []schema.Tag{}
	for _, entry := range entries {
		tag := entry.Value.Data()
		if tag.Network == netID {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// UpdateTag - updates and syncs hosts with tag update
func UpdateTag(req models.UpdateTagReq, newID schema.TagID) {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	network := &schema.Network{
		Name: req.Network.String(),
	}
	err := network.Get(db.WithContext(context.TODO()))
	if err != nil {
		return
	}

	var taggedNodeIDs []interface{}
	taggedExtclientIDs := map[string]struct{}{}
	for _, node := range req.TaggedNodes {
		if !node.IsStatic {
			taggedNodeIDs = append(taggedNodeIDs, node.ID)
		} else {
			if node.StaticNode.RemoteAccessClientID != "" {
				continue
			}
			taggedExtclientIDs[node.StaticNode.ClientID] = struct{}{}
		}
	}

	_ = (&schema.Node{}).UnassignTag(
		db.WithContext(context.TODO()),
		req.ID.String(),
		dbtypes.WithFilter("network_id", network.ID),
	)

	tagID := req.ID
	if newID != "" {
		tagID = newID
	}

	// WithFilter skips adding the filter when no values are passed.
	// So, even though it is expected to work, the effective query
	// that's executed assigns the tag to all the nodes in the network.
	// To avoid that ensure the taggedNodeIDs is non-empty.
	if len(taggedNodeIDs) > 0 {
		_ = (&schema.Node{}).AssignTag(
			db.WithContext(context.TODO()),
			tagID.String(),
			dbtypes.WithFilter("network_id", network.ID),
			dbtypes.WithFilter("id", taggedNodeIDs...),
		)
	}

	extclients, _ := logic.GetNetworkExtClients(req.Network.String())
	for _, extclient := range extclients {
		if extclient.Tags == nil {
			extclient.Tags = make(map[schema.TagID]struct{})
		}

		// unassign old tag
		if _, ok := extclient.Tags[req.ID]; ok {
			if newID != "" {
				delete(extclient.Tags, req.ID)
			}
		}

		// assign tag if in taggedExtclientIDs.
		if _, ok := taggedExtclientIDs[extclient.ClientID]; ok {
			extclient.Tags[tagID] = struct{}{}
		}
		_ = logic.SaveExtClient(&extclient)
	}
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
	reg, err := regexp.Compile("^[a-zA-Z0-9- ]+$")
	if err != nil {
		return err
	}
	if !reg.MatchString(id) {
		return errors.New("invalid name. allowed characters are [a-zA-Z-]")
	}
	return nil
}

func CreateDefaultTags(netID schema.NetworkID) {
	// create tag for gws in the network
	tag := schema.Tag{
		ID:        schema.TagID(fmt.Sprintf("%s.%s", netID.String(), schema.GwTagName)),
		TagName:   schema.GwTagName,
		Network:   netID,
		CreatedBy: "auto",
		CreatedAt: time.Now().UTC(),
	}
	_, err := GetTag(tag.ID)
	if err == nil {
		return
	}
	err = InsertTag(tag)
	if err != nil {
		slog.Error("failed to create gw tag", "error", err.Error())
		return
	}
}
