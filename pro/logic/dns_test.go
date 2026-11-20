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
	groups := map[schema.UserGroupID]struct{}{"eng": {}}

	tests := []struct {
		name   string
		ns     schema.Nameserver
		owner  string
		groups map[schema.UserGroupID]struct{}
		want   bool
	}{
		{"all resources", schema.Nameserver{Tags: datatypes.JSONMap{"*": ""}}, "", nil, true},
		{"all resources skips user device", schema.Nameserver{Tags: datatypes.JSONMap{"*": ""}}, "alice", nil, false},
		{"user device keeps its tags", schema.Nameserver{Tags: datatypes.JSONMap{"*": "", "net.dev": ""}}, "alice", nil, true},
		{"tag", schema.Nameserver{Tags: datatypes.JSONMap{"net.dev": ""}}, "", nil, true},
		{"node", schema.Nameserver{Nodes: datatypes.JSONMap{node.ID.String(): ""}}, "", nil, true},
		{"owner user", schema.Nameserver{Users: datatypes.JSONMap{"alice": ""}}, "alice", nil, true},
		{"all users", schema.Nameserver{Users: datatypes.JSONMap{"*": ""}}, "alice", nil, true},
		{"all users skips unowned node", schema.Nameserver{Users: datatypes.JSONMap{"*": ""}}, "", nil, false},
		{"other user", schema.Nameserver{Users: datatypes.JSONMap{"bob": ""}}, "alice", nil, false},
		{"owner group", schema.Nameserver{UserGroups: datatypes.JSONMap{"eng": ""}}, "alice", groups, true},
		{"other group", schema.Nameserver{UserGroups: datatypes.JSONMap{"ops": ""}}, "alice", groups, false},
		{"unowned node ignores users and groups", schema.Nameserver{
			Users:      datatypes.JSONMap{"alice": ""},
			UserGroups: datatypes.JSONMap{"eng": ""},
		}, "", nil, false},
		{"no targets", schema.Nameserver{}, "alice", nil, false},
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
