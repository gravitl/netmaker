package pro

import (
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/stretchr/testify/assert"
)

func TestNetworkProSettings(t *testing.T) {
	t.Run("Uninitialized with pro", func(t *testing.T) {
		network := models.Network{
			NetID: "helloworld",
		}
		assert.Nil(t, network.ProSettings)
	})
	t.Run("Initialized with pro", func(t *testing.T) {
		network := models.Network{
			NetID: "helloworld",
		}
		AddProNetDefaults(&network)
		assert.NotNil(t, network.ProSettings)
	})
	t.Run("Net Zero Defaults set correctly with Pro", func(t *testing.T) {
		network := models.Network{
			NetID: "helloworld",
		}
		AddProNetDefaults(&network)
		assert.NotNil(t, network.ProSettings)
		assert.Equal(t, NO_ACCESS, network.ProSettings.DefaultAccessLevel)
		assert.Equal(t, 0, network.ProSettings.DefaultUserClientLimit)
		assert.Equal(t, 0, network.ProSettings.DefaultUserNodeLimit)
	})
	t.Run("Net Defaults set correctly with Pro", func(t *testing.T) {
		network := models.Network{
			NetID: "helloworld",
			ProSettings: &promodels.ProNetwork{
				DefaultAccessLevel:     NET_ADMIN,
				DefaultUserNodeLimit:   10,
				DefaultUserClientLimit: 25,
			},
		}
		AddProNetDefaults(&network)
		assert.NotNil(t, network.ProSettings)
		assert.Equal(t, NET_ADMIN, network.ProSettings.DefaultAccessLevel)
		assert.Equal(t, 25, network.ProSettings.DefaultUserClientLimit)
		assert.Equal(t, 10, network.ProSettings.DefaultUserNodeLimit)
	})
	t.Run("Net Defaults set to allow all groups/users", func(t *testing.T) {
		network := models.Network{
			NetID: "helloworld",
			ProSettings: &promodels.ProNetwork{
				DefaultAccessLevel:     NET_ADMIN,
				DefaultUserNodeLimit:   10,
				DefaultUserClientLimit: 25,
			},
		}
		AddProNetDefaults(&network)
		assert.NotNil(t, network.ProSettings)
		assert.Equal(t, len(network.ProSettings.AllowedGroups), 1)
		assert.Equal(t, len(network.ProSettings.AllowedUsers), 0)
	})
}
