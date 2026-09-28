package logic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

var tagMutex = &sync.RWMutex{}

// GetTag - fetches tag info
func GetTag(ctx context.Context, tagID models.TagID) (models.Tag, error) {
	network, name, ok := strings.Cut(tagID.String(), ".")
	if !ok {
		return models.Tag{}, gorm.ErrRecordNotFound
	}
	_tag := &schema.Tag{
		Name:    name,
		Network: &schema.Network{Name: network},
	}
	if err := _tag.Get(ctx); err != nil {
		return models.Tag{}, err
	}
	return ConvertSchemaTagToModelsTag(_tag), nil
}

// UpsertTag - updates the tag, creating it if it does not exist
func UpsertTag(ctx context.Context, tag models.Tag) error {
	_tag, err := ConvertModelsTagToSchemaTag(ctx, tag)
	if err != nil {
		return err
	}
	existing := &schema.Tag{NetworkID: _tag.NetworkID, Name: _tag.Name}
	if err := existing.Get(ctx); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return _tag.Create(ctx)
	}
	_tag.ID = existing.ID
	return _tag.Update(ctx)
}

// InsertTag - creates new tag
func InsertTag(ctx context.Context, tag models.Tag) error {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	if _, err := GetTag(ctx, tag.ID); err == nil {
		return fmt.Errorf("tag `%s` exists already", tag.ID)
	}
	_tag, err := ConvertModelsTagToSchemaTag(ctx, tag)
	if err != nil {
		return err
	}
	return _tag.Create(ctx)
}

// ConvertSchemaTagToModelsTag - converts the tag, its Network must be set
func ConvertSchemaTagToModelsTag(_tag *schema.Tag) models.Tag {
	var network string
	if _tag.Network != nil {
		network = _tag.Network.Name
	}
	return models.Tag{
		ID:        models.TagID(fmt.Sprintf("%s.%s", network, _tag.Name)),
		TagName:   _tag.Name,
		Network:   schema.NetworkID(network),
		ColorCode: _tag.ColorCode,
		CreatedBy: _tag.CreatedBy,
		CreatedAt: _tag.CreatedAt,
	}
}

// ConvertModelsTagToSchemaTag - converts the tag, resolving its network by
// name within the tenant in the context
func ConvertModelsTagToSchemaTag(ctx context.Context, tag models.Tag) (*schema.Tag, error) {
	network := &schema.Network{Name: tag.Network.String()}
	if err := network.Get(ctx); err != nil {
		return nil, err
	}
	return &schema.Tag{
		TenantID:  scope.ID(ctx),
		NetworkID: network.ID,
		Network:   network,
		Name:      tag.TagName,
		ColorCode: tag.ColorCode,
		CreatedBy: tag.CreatedBy,
		CreatedAt: tag.CreatedAt,
	}, nil
}

// DeleteTag - delete tag, will also untag hosts
func DeleteTag(ctx context.Context, tagID models.TagID, removeFromPolicy bool) error {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	// cleanUp tags on hosts
	tag, err := GetTag(ctx, tagID)
	if err != nil {
		return err
	}
	network := &schema.Network{
		Name: tag.Network.String(),
	}
	err = network.Get(ctx)
	if err != nil {
		return err
	}

	_ = (&schema.Node{}).UnassignTag(
		ctx,
		tag.ID.String(),
		dbtypes.WithFilter("network_id", network.ID),
	)

	if removeFromPolicy {
		// remove tag used on acl policy
		go RemoveDeviceTagFromAclPolicies(ctx, tagID, tag.Network)
	}
	go RemoveTagFromEgress(tag.Network, tagID)
	extclients, _ := logic.GetNetworkExtClients(ctx, tag.Network.String())
	for _, extclient := range extclients {
		if _, ok := extclient.Tags[tagID]; ok {
			delete(extclient.Tags, tagID)
			logic.SaveExtClient(ctx, &extclient)
		}
	}
	return (&schema.Tag{NetworkID: network.ID, Name: tag.TagName}).Delete(ctx)
}

// ListTagsWithHosts - lists all tags with tagged hosts
func ListTagsWithNodes(ctx context.Context, netID schema.NetworkID) ([]models.TagListResp, error) {
	tags, err := ListNetworkTags(ctx, netID)
	if err != nil {
		return []models.TagListResp{}, err
	}
	tagsNodeMap := GetTagMapWithNodesByNetwork(ctx, netID, true)
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
func DeleteAllNetworkTags(ctx context.Context, networkID schema.NetworkID) {
	tags, _ := ListNetworkTags(ctx, networkID)
	for _, tagI := range tags {
		DeleteTag(ctx, tagI.ID, false)
	}
}

// ListNetworkTags - lists all tags in network
func ListNetworkTags(ctx context.Context, netID schema.NetworkID) ([]models.Tag, error) {
	tagMutex.RLock()
	defer tagMutex.RUnlock()
	_tags, err := (&schema.Tag{Network: &schema.Network{Name: netID.String()}}).ListByNetwork(ctx)
	if err != nil {
		return []models.Tag{}, err
	}
	tags := make([]models.Tag, 0, len(_tags))
	for i := range _tags {
		tags = append(tags, ConvertSchemaTagToModelsTag(&_tags[i]))
	}
	return tags, nil
}

// UpdateTag - updates and syncs hosts with tag update
func UpdateTag(ctx context.Context, req models.UpdateTagReq, newID models.TagID) {
	tagMutex.Lock()
	defer tagMutex.Unlock()
	network := &schema.Network{
		Name: req.Network.String(),
	}
	err := network.Get(ctx)
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
		ctx,
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
			ctx,
			tagID.String(),
			dbtypes.WithFilter("network_id", network.ID),
			dbtypes.WithFilter("id", taggedNodeIDs...),
		)
	}

	extclients, _ := logic.GetNetworkExtClients(ctx, req.Network.String())
	for _, extclient := range extclients {
		if extclient.Tags == nil {
			extclient.Tags = make(map[models.TagID]struct{})
		}

		// unassign old tag
		delete(extclient.Tags, req.ID)

		// assign tag if in taggedExtclientIDs.
		if _, ok := taggedExtclientIDs[extclient.ClientID]; ok {
			extclient.Tags[tagID] = struct{}{}
		}
		_ = logic.SaveExtClient(ctx, &extclient)
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

func CreateDefaultTags(ctx context.Context, netID schema.NetworkID) {
	// create tag for gws in the network
	tag := models.Tag{
		ID:        models.TagID(fmt.Sprintf("%s.%s", netID.String(), models.GwTagName)),
		TagName:   models.GwTagName,
		Network:   netID,
		CreatedBy: "auto",
		CreatedAt: time.Now().UTC(),
	}
	_, err := GetTag(ctx, tag.ID)
	if err == nil {
		return
	}
	err = InsertTag(ctx, tag)
	if err != nil {
		slog.Error("failed to create gw tag", "error", err.Error())
		return
	}
}
