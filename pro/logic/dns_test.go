package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestNameserverTargetsNode(t *testing.T) {
	node := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "net"},
		Tags:       map[models.TagID]struct{}{"net.dev": {}},
	}
	groups := func() map[schema.UserGroupID]struct{} {
		return map[schema.UserGroupID]struct{}{"eng": {}}
	}
	noGroupsCalled := func() map[schema.UserGroupID]struct{} {
		t.Fatal("owner groups should not be loaded")
		return nil
	}

	tests := []struct {
		name   string
		ns     schema.Nameserver
		owner  string
		groups func() map[schema.UserGroupID]struct{}
		want   bool
	}{
		{"all", schema.Nameserver{Tags: datatypes.JSONMap{"*": ""}}, "", noGroupsCalled, true},
		{"tag", schema.Nameserver{Tags: datatypes.JSONMap{"net.dev": ""}}, "", noGroupsCalled, true},
		{"node", schema.Nameserver{Nodes: datatypes.JSONMap{node.ID.String(): ""}}, "", noGroupsCalled, true},
		{"owner user", schema.Nameserver{Users: datatypes.JSONMap{"alice": ""}}, "alice", noGroupsCalled, true},
		{"all users", schema.Nameserver{Users: datatypes.JSONMap{"*": ""}}, "alice", noGroupsCalled, true},
		{"all users skips unowned node", schema.Nameserver{Users: datatypes.JSONMap{"*": ""}}, "", noGroupsCalled, false},
		{"other user", schema.Nameserver{Users: datatypes.JSONMap{"bob": ""}}, "alice", noGroupsCalled, false},
		{"owner group", schema.Nameserver{UserGroups: datatypes.JSONMap{"eng": ""}}, "alice", groups, true},
		{"other group", schema.Nameserver{UserGroups: datatypes.JSONMap{"ops": ""}}, "alice", groups, false},
		{"unowned node ignores users and groups", schema.Nameserver{
			Users:      datatypes.JSONMap{"alice": ""},
			UserGroups: datatypes.JSONMap{"eng": ""},
		}, "", noGroupsCalled, false},
		{"no targets", schema.Nameserver{}, "alice", noGroupsCalled, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nameserverTargetsNode(&tt.ns, node, tt.owner, tt.groups))
		})
	}
}

func TestValidateNameserverReq_allUsersIsExclusive(t *testing.T) {
	ns := &schema.Nameserver{
		Name:      "ns",
		NetworkID: "net",
		Servers:   []string{"10.0.0.53"},
		Users:     datatypes.JSONMap{"*": "", "alice": ""},
	}
	assert.EqualError(t, ValidateNameserverReq(context.Background(), ns),
		"all users (*) cannot be combined with specific users")
}
