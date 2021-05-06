package controller

import (
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
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

func TestCreateNode(t *testing.T) {
}
func TestDeleteNode(t *testing.T) {
}
func TestGetNode(t *testing.T) {
}
func TestGetPeerList(t *testing.T) {
}
func TestNodeCheckIn(t *testing.T) {
}
func TestSetNetworkNodesLastModified(t *testing.T) {
}
func TestTimestampNode(t *testing.T) {
}
func TestUpdateNode(t *testing.T) {
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
		assert.NotContains(t, err.Error(), "Field validation for 'Address' failed on the 'address_check' tag")
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
			errorMessage: "Field validation for 'Address' failed on the 'address_check' tag",
		},
		NodeValidationUpdateTC{
			testname: "BadAddress6",
			node: models.NodeUpdate{
				Address6: "2607::abcd:efgh::1",
			},
			errorMessage: "Field validation for 'Address6' failed on the 'address6_check' tag",
		},
		NodeValidationUpdateTC{
			testname: "BadLocalAddress",
			node: models.NodeUpdate{
				LocalAddress: "10.0.200.300",
			},
			errorMessage: "Field validation for 'LocalAddress' failed on the 'localaddress_check' tag",
		},
		NodeValidationUpdateTC{
			testname: "InvalidName",
			node: models.NodeUpdate{
				Name: "mynode*",
			},
			errorMessage: "Field validation for 'Name' failed on the 'name_valid' tag",
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
				PublicKey: "",
			},
			errorMessage: "Field validation for 'PublicKey' failed on the 'pubkey_check' tag",
		},
		NodeValidationUpdateTC{
			testname: "EndpointInvalid",
			node: models.NodeUpdate{
				Endpoint: "10.2.0.300",
			},
			errorMessage: "Field validation for 'Endpoint' failed on the 'endpoint_check' tag",
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
			errorMessage: "Field validation for 'MacAddress' failed on the 'macaddress_valid' tag",
		},
		NodeValidationUpdateTC{
			testname: "MacAddressMissing",
			node: models.NodeUpdate{
				MacAddress: "",
			},
			errorMessage: "Field validation for 'MacAddress' failed on the 'required' tag",
		},
		NodeValidationUpdateTC{
			testname: "EmptyPassword",
			node: models.NodeUpdate{
				Password: "",
			},
			errorMessage: "Field validation for 'Password' failed on the 'password_check' tag",
		},
		NodeValidationUpdateTC{
			testname: "ShortPassword",
			node: models.NodeUpdate{
				Password: "1234",
			},
			errorMessage: "Field validation for 'Password' failed on the 'password_check' tag",
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
