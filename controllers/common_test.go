package controller

import (
	"context"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

type NodeValidationTC struct {
	testname     string
	node         models.Node
	errorMessage string
}

type NodeValidationUpdateTC struct {
	testname     string
	node         models.NodeUpdate
	errorMessage string
}

func createTestNode() models.Node {
	createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet"}
	node, err := CreateNode(createnode, "skynet")
	if err != nil {
		panic(err)
	}
	return node
}

func TestCreateNode(t *testing.T) {
	deleteNet(t)
	createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet"}
	createNet()
	err := ValidateNodeCreate("skynet", createnode)
	assert.Nil(t, err)
	node, err := CreateNode(createnode, "skynet")
	assert.Nil(t, err)
	assert.Equal(t, "10.0.0.1", node.Endpoint)
	assert.Equal(t, "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", node.PublicKey)
	assert.Equal(t, "01:02:03:04:05:06", node.MacAddress)
	assert.Equal(t, int32(51821), node.ListenPort)
	assert.NotNil(t, node.Name)
	assert.Equal(t, "skynet", node.Network)
	assert.Equal(t, "nm-skynet", node.Interface)
}
func TestDeleteNode(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode()
	t.Run("NodeExists", func(t *testing.T) {
		deleted, err := DeleteNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.True(t, deleted)
	})
	t.Run("NonExistantNode", func(t *testing.T) {
		deleted, err := DeleteNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.False(t, deleted)
	})
}
func TestGetNode(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode()
	t.Run("NodeExists", func(t *testing.T) {
		response, err := GetNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.Equal(t, "10.0.0.1", response.Endpoint)
		assert.Equal(t, "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", response.PublicKey)
		assert.Equal(t, "01:02:03:04:05:06", response.MacAddress)
		assert.Equal(t, int32(51821), response.ListenPort)
		assert.NotNil(t, response.Name)
		assert.Equal(t, "skynet", response.Network)
		assert.Equal(t, "nm-skynet", response.Interface)
	})
	t.Run("BadMac", func(t *testing.T) {
		response, err := GetNode("01:02:03:04:05:07", node.Network)
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, response)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("BadNetwork", func(t *testing.T) {
		response, err := GetNode(node.MacAddress, "badnet")
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, response)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("NoNode", func(t *testing.T) {
		_, _ = DeleteNode("01:02:03:04:05:06", "skynet")
		response, err := GetNode(node.MacAddress, node.Network)
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, response)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})

}
func TestGetPeerList(t *testing.T) {
	deleteNet(t)
	createNet()
	_ = createTestNode()
	//createnode := models.Node{PublicKey: "RM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.2", MacAddress: "02:02:03:04:05:06", Password: "password", Network: "skynet"}
	//_, _ = CreateNode(createnode, "skynet")
	t.Run("PeerExist", func(t *testing.T) {
		peers, err := GetPeersList("skynet")
		assert.Nil(t, err)
		assert.NotEqual(t, []models.PeersResponse(nil), peers)
		t.Log(peers)
	})
	t.Run("NoNodes", func(t *testing.T) {
		_, _ = DeleteNode("01:02:03:04:05:06", "skynet")
		peers, err := GetPeersList("skynet")
		assert.Nil(t, err)
		assert.Equal(t, []models.PeersResponse(nil), peers)
		t.Log(peers)
	})
}
func TestNodeCheckIn(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode()
	time.Sleep(time.Second * 1)
	expectedResponse := models.CheckInResponse{false, false, false, false, false, "", false}
	t.Run("BadNet", func(t *testing.T) {
		response, err := NodeCheckIn(node, "badnet")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Couldnt retrieve Network badnet: ")
		assert.Equal(t, expectedResponse, response)
	})
	t.Run("BadNode", func(t *testing.T) {
		badnode := models.Node{PublicKey: "RM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.2", MacAddress: "02:02:03:04:05:06", Password: "password", Network: "skynet"}
		response, err := NodeCheckIn(badnode, "skynet")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Couldnt Get Node 02:02:03:04:05:06")
		assert.Equal(t, expectedResponse, response)
	})
	t.Run("NoUpdatesNeeded", func(t *testing.T) {
		expectedResponse := models.CheckInResponse{true, false, false, false, false, "", false}
		response, err := NodeCheckIn(node, node.Network)
		assert.Nil(t, err)
		assert.Equal(t, expectedResponse, response)
	})
	t.Run("NodePending", func(t *testing.T) {
		//		create Pending Node
		createnode := models.Node{PublicKey: "RM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.2", MacAddress: "01:02:03:04:05:07", Password: "password", Network: "skynet", IsPending: true}
		pendingNode, _ := CreateNode(createnode, "skynet")
		expectedResponse.IsPending = true
		response, err := NodeCheckIn(pendingNode, "skynet")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Node checking in is still pending: 01:02:03:04:05:07")
		assert.Equal(t, expectedResponse, response)
	})
	t.Run("ConfigUpdateRequired", func(t *testing.T) {
		err := TimestampNode(node, false, false, true)
		assert.Nil(t, err)
		expectedResponse.NeedConfigUpdate = true
		expectedResponse.Success = true
		response, err := NodeCheckIn(node, "skynet")
		assert.Nil(t, err)
		assert.Equal(t, true, response.Success)
		assert.Equal(t, true, response.NeedConfigUpdate)
	})
	t.Run("PeerUpdateRequired", func(t *testing.T) {
		var nodeUpdate models.NodeUpdate
		newtime := time.Now().Add(time.Hour * -24).Unix()
		nodeUpdate.LastPeerUpdate = newtime
		_, err := UpdateNode(nodeUpdate, node)
		assert.Nil(t, err)
		response, err := NodeCheckIn(node, "skynet")
		assert.Nil(t, err)
		assert.Equal(t, true, response.Success)
		assert.Equal(t, true, response.NeedPeerUpdate)
	})
	t.Run("KeyUpdateRequired", func(t *testing.T) {
		var network models.Network
		newtime := time.Now().Add(time.Hour * 24).Unix()
		t.Log(newtime, time.Now().Unix())
		//this is cheating; but can't find away to update timestamp through existing api
		collection := mongoconn.Client.Database("netmaker").Collection("networks")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		filter := bson.M{"netid": "skynet"}
		update := bson.D{
			{"$set", bson.D{
				{"keyupdatetimestamp", newtime},
			}},
		}
		defer cancel()
		err := collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)
		assert.Nil(t, err)
		response, err := NodeCheckIn(node, "skynet")
		assert.Nil(t, err)
		assert.Equal(t, true, response.Success)
		assert.Equal(t, true, response.NeedKeyUpdate)
	})
	t.Run("DeleteNeeded", func(t *testing.T) {
		var nodeUpdate models.NodeUpdate
		newtime := time.Now().Add(time.Hour * -24).Unix()
		nodeUpdate.ExpirationDateTime = newtime
		_, err := UpdateNode(nodeUpdate, node)
		assert.Nil(t, err)
		response, err := NodeCheckIn(node, "skynet")
		assert.Nil(t, err)
		assert.Equal(t, true, response.Success)
		assert.Equal(t, true, response.NeedDelete)
	})
}

func TestSetNetworkNodesLastModified(t *testing.T) {
	deleteNet(t)
	createNet()
	t.Run("InvalidNetwork", func(t *testing.T) {
		err := SetNetworkNodesLastModified("badnet")
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("NetworkExists", func(t *testing.T) {
		err := SetNetworkNodesLastModified("skynet")
		assert.Nil(t, err)
	})
}
func TestTimestampNode(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode()
	time.Sleep(time.Second * 1)
	before, err := GetNode(node.MacAddress, node.Network)
	assert.Nil(t, err)
	t.Run("UpdateCheckIn", func(t *testing.T) {
		err = TimestampNode(node, true, false, false)
		assert.Nil(t, err)
		after, err := GetNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.Greater(t, after.LastCheckIn, before.LastCheckIn)
	})
	t.Run("UpdatePeers", func(t *testing.T) {
		err = TimestampNode(node, false, true, false)
		assert.Nil(t, err)
		after, err := GetNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.Greater(t, after.LastPeerUpdate, before.LastPeerUpdate)
	})
	t.Run("UpdateLastModified", func(t *testing.T) {
		err = TimestampNode(node, false, false, true)
		assert.Nil(t, err)
		after, err := GetNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.Greater(t, after.LastModified, before.LastModified)
	})
	t.Run("InvalidNode", func(t *testing.T) {
		node.MacAddress = "01:02:03:04:05:08"
		err = TimestampNode(node, true, true, true)
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})

}
func TestUpdateNode(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode()
	var update models.NodeUpdate
	update.MacAddress = "01:02:03:04:05:06"
	update.Name = "helloworld"
	newnode, err := UpdateNode(update, node)
	assert.Nil(t, err)
	assert.Equal(t, update.Name, newnode.Name)
}

func TestValidateNodeCreate(t *testing.T) {
	cases := []NodeValidationTC{
		//		NodeValidationTC{
		//			testname: "EmptyAddress",
		//			node: models.Node{
		//				Address: "",
		//			},
		//			errorMessage: "Field validation for 'Endpoint' failed on the 'address_check' tag",
		//		},
		NodeValidationTC{
			testname: "BadAddress",
			node: models.Node{
				Address: "256.0.0.1",
			},
			errorMessage: "Field validation for 'Address' failed on the 'ipv4' tag",
		},
		NodeValidationTC{
			testname: "BadAddress6",
			node: models.Node{
				Address6: "2607::abcd:efgh::1",
			},
			errorMessage: "Field validation for 'Address6' failed on the 'ipv6' tag",
		},
		NodeValidationTC{
			testname: "BadLocalAddress",
			node: models.Node{
				LocalAddress: "10.0.200.300",
			},
			errorMessage: "Field validation for 'LocalAddress' failed on the 'ip' tag",
		},
		NodeValidationTC{
			testname: "InvalidName",
			node: models.Node{
				Name: "mynode*",
			},
			errorMessage: "Field validation for 'Name' failed on the 'alphanum' tag",
		},
		NodeValidationTC{
			testname: "NameTooLong",
			node: models.Node{
				Name: "mynodexmynode",
			},
			errorMessage: "Field validation for 'Name' failed on the 'max' tag",
		},
		NodeValidationTC{
			testname: "ListenPortMin",
			node: models.Node{
				ListenPort: 1023,
			},
			errorMessage: "Field validation for 'ListenPort' failed on the 'min' tag",
		},
		NodeValidationTC{
			testname: "ListenPortMax",
			node: models.Node{
				ListenPort: 65536,
			},
			errorMessage: "Field validation for 'ListenPort' failed on the 'max' tag",
		},
		NodeValidationTC{
			testname: "PublicKeyEmpty",
			node: models.Node{
				PublicKey: "",
			},
			errorMessage: "Field validation for 'PublicKey' failed on the 'required' tag",
		},
		NodeValidationTC{
			testname: "PublicKeyInvalid",
			node: models.Node{
				PublicKey: "junk%key",
			},
			errorMessage: "Field validation for 'PublicKey' failed on the 'base64' tag",
		},
		NodeValidationTC{
			testname: "EndpointInvalid",
			node: models.Node{
				Endpoint: "10.2.0.300",
			},
			errorMessage: "Field validation for 'Endpoint' failed on the 'ip' tag",
		},
		NodeValidationTC{
			testname: "EndpointEmpty",
			node: models.Node{
				Endpoint: "",
			},
			errorMessage: "Field validation for 'Endpoint' failed on the 'required' tag",
		},
		NodeValidationTC{
			testname: "PersistentKeepaliveMax",
			node: models.Node{
				PersistentKeepalive: 1001,
			},
			errorMessage: "Field validation for 'PersistentKeepalive' failed on the 'max' tag",
		},
		NodeValidationTC{
			testname: "MacAddressInvalid",
			node: models.Node{
				MacAddress: "01:02:03:04:05",
			},
			errorMessage: "Field validation for 'MacAddress' failed on the 'mac' tag",
		},
		NodeValidationTC{
			testname: "MacAddressMissing",
			node: models.Node{
				MacAddress: "",
			},
			errorMessage: "Field validation for 'MacAddress' failed on the 'required' tag",
		},
		NodeValidationTC{
			testname: "EmptyPassword",
			node: models.Node{
				Password: "",
			},
			errorMessage: "Field validation for 'Password' failed on the 'required' tag",
		},
		NodeValidationTC{
			testname: "ShortPassword",
			node: models.Node{
				Password: "1234",
			},
			errorMessage: "Field validation for 'Password' failed on the 'min' tag",
		},
		NodeValidationTC{
			testname: "NoNetwork",
			node: models.Node{
				Network: "badnet",
			},
			errorMessage: "Field validation for 'Network' failed on the 'network_exists' tag",
		},
	}

	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			err := ValidateNodeCreate("skynet", tc.node)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errorMessage)
		})
	}
	t.Run("MacAddresUnique", func(t *testing.T) {
		createNet()
		node := models.Node{MacAddress: "01:02:03:04:05:06", Network: "skynet"}
		_, err := CreateNode(node, "skynet")
		assert.Nil(t, err)
		err = ValidateNodeCreate("skynet", node)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'MacAddress' failed on the 'macaddress_unique' tag")
	})
	t.Run("EmptyAddress", func(t *testing.T) {
		node := models.Node{Address: ""}
		err := ValidateNodeCreate("skynet", node)
		assert.NotNil(t, err)
		assert.NotContains(t, err.Error(), "Field validation for 'Address' failed on the 'ipv4' tag")
	})
}
func TestValidateNodeUpdate(t *testing.T) {
	//cases
	cases := []NodeValidationUpdateTC{
		NodeValidationUpdateTC{
			testname: "BadAddress",
			node: models.NodeUpdate{
				Address: "256.0.0.1",
			},
			errorMessage: "Field validation for 'Address' failed on the 'ip' tag",
		},
		NodeValidationUpdateTC{
			testname: "BadAddress6",
			node: models.NodeUpdate{
				Address6: "2607::abcd:efgh::1",
			},
			errorMessage: "Field validation for 'Address6' failed on the 'ipv6' tag",
		},
		NodeValidationUpdateTC{
			testname: "BadLocalAddress",
			node: models.NodeUpdate{
				LocalAddress: "10.0.200.300",
			},
			errorMessage: "Field validation for 'LocalAddress' failed on the 'ip' tag",
		},
		NodeValidationUpdateTC{
			testname: "InvalidName",
			node: models.NodeUpdate{
				Name: "mynode*",
			},
			errorMessage: "Field validation for 'Name' failed on the 'alphanum' tag",
		},
		NodeValidationUpdateTC{
			testname: "NameTooLong",
			node: models.NodeUpdate{
				Name: "mynodexmynode",
			},
			errorMessage: "Field validation for 'Name' failed on the 'max' tag",
		},
		NodeValidationUpdateTC{
			testname: "ListenPortMin",
			node: models.NodeUpdate{
				ListenPort: 1023,
			},
			errorMessage: "Field validation for 'ListenPort' failed on the 'min' tag",
		},
		NodeValidationUpdateTC{
			testname: "ListenPortMax",
			node: models.NodeUpdate{
				ListenPort: 65536,
			},
			errorMessage: "Field validation for 'ListenPort' failed on the 'max' tag",
		},
		NodeValidationUpdateTC{
			testname: "PublicKeyInvalid",
			node: models.NodeUpdate{
				PublicKey: "bad&key",
			},
			errorMessage: "Field validation for 'PublicKey' failed on the 'base64' tag",
		},
		NodeValidationUpdateTC{
			testname: "EndpointInvalid",
			node: models.NodeUpdate{
				Endpoint: "10.2.0.300",
			},
			errorMessage: "Field validation for 'Endpoint' failed on the 'ip' tag",
		},
		NodeValidationUpdateTC{
			testname: "PersistentKeepaliveMax",
			node: models.NodeUpdate{
				PersistentKeepalive: 1001,
			},
			errorMessage: "Field validation for 'PersistentKeepalive' failed on the 'max' tag",
		},
		NodeValidationUpdateTC{
			testname: "MacAddressInvalid",
			node: models.NodeUpdate{
				MacAddress: "01:02:03:04:05",
			},
			errorMessage: "Field validation for 'MacAddress' failed on the 'mac' tag",
		},
		NodeValidationUpdateTC{
			testname: "MacAddressMissing",
			node: models.NodeUpdate{
				MacAddress: "",
			},
			errorMessage: "Field validation for 'MacAddress' failed on the 'required' tag",
		},
		NodeValidationUpdateTC{
			testname: "ShortPassword",
			node: models.NodeUpdate{
				Password: "1234",
			},
			errorMessage: "Field validation for 'Password' failed on the 'min' tag",
		},
		NodeValidationUpdateTC{
			testname: "NoNetwork",
			node: models.NodeUpdate{
				Network: "badnet",
			},
			errorMessage: "Field validation for 'Network' failed on the 'network_exists' tag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			err := ValidateNodeUpdate("skynet", tc.node)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errorMessage)
		})
	}

}
