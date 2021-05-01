package controller

import (
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

type NetworkValidationTestCase struct {
	testname   string
	network    models.Network
	errMessage string
}

func TestGetNetworks(t *testing.T) {
	//calls functions.ListNetworks --- nothing to be don
}
func TestCreateNetwork(t *testing.T) {
}
func TestGetNetwork(t *testing.T) {
}
func TestUpdateNetwork(t *testing.T) {
}
func TestDeleteNetwork(t *testing.T) {
}
func TestKeyUpdate(t *testing.T) {
}
func TestCreateKey(t *testing.T) {
}
func TestGetKey(t *testing.T) {
}
func TestDeleteKey(t *testing.T) {
}
func TestSecurityCheck(t *testing.T) {
}
func TestValidateNetworkUpdate(t *testing.T) {
}
func TestValidateNetworkCreate(t *testing.T) {
	yes := true
	no := false
	//DeleteNetworks
	cases := []NetworkValidationTestCase{
		NetworkValidationTestCase{
			testname: "InvalidAddress",
			network: models.Network{
				AddressRange: "10.0.0.256",
				NetID:        "skynet",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'AddressRange' failed on the 'cidr' tag",
		},
		NetworkValidationTestCase{
			testname: "BadDisplayName",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				DisplayName:  "skynet*",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'DisplayName' failed on the 'alphanum' tag",
		},
		NetworkValidationTestCase{
			testname: "DisplayNameTooLong",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				DisplayName:  "Thisisareallylongdisplaynamethatistoolong",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'DisplayName' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "DisplayNameTooShort",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				DisplayName:  "1",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'DisplayName' failed on the 'min' tag",
		},
		NetworkValidationTestCase{
			testname: "NetIDMissing",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'required' tag",
		},
		NetworkValidationTestCase{
			testname: "InvalidNetID",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "contains spaces",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'alphanum' tag",
		},
		NetworkValidationTestCase{
			testname: "NetIDTooShort",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'required' tag",
		},
		NetworkValidationTestCase{
			testname: "NetIDTooLong",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "LongNetIDName",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "ListenPortTooLow",
			network: models.Network{
				AddressRange:      "10.0.0.1/24",
				NetID:             "skynet",
				DefaultListenPort: 1023,
				IsDualStack:       &no,
			},
			errMessage: "Field validation for 'DefaultListenPort' failed on the 'min' tag",
		},
		NetworkValidationTestCase{
			testname: "ListenPortTooHigh",
			network: models.Network{
				AddressRange:      "10.0.0.1/24",
				NetID:             "skynet",
				DefaultListenPort: 65536,
				IsDualStack:       &no,
			},
			errMessage: "Field validation for 'DefaultListenPort' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "KeepAliveTooBig",
			network: models.Network{
				AddressRange:     "10.0.0.1/24",
				NetID:            "skynet",
				DefaultKeepalive: 1010,
				IsDualStack:      &no,
			},
			errMessage: "Field validation for 'DefaultKeepalive' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "InvalidLocalRange",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				LocalRange:   "192.168.0.1",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'LocalRange' failed on the 'cidr' tag",
		},
		NetworkValidationTestCase{
			testname: "DualStackWithoutIPv6",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				IsDualStack:  &yes,
			},
			errMessage: "Field validation for 'AddressRange6' failed on the 'addressrange6_valid' tag",
		},
		NetworkValidationTestCase{
			testname: "CheckInIntervalTooBig",
			network: models.Network{
				AddressRange:           "10.0.0.1/24",
				NetID:                  "skynet",
				IsDualStack:            &no,
				DefaultCheckInInterval: 100001,
			},
			errMessage: "Field validation for 'DefaultCheckInInterval' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "CheckInIntervalTooSmall",
			network: models.Network{
				AddressRange:           "10.0.0.1/24",
				NetID:                  "skynet",
				IsDualStack:            &no,
				DefaultCheckInInterval: 1,
			},
			errMessage: "Field validation for 'DefaultCheckInInterval' failed on the 'min' tag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			err := ValidateNetworkCreate(tc.network)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errMessage)
		})
	}
	t.Run("DuplicateNetID", func(t *testing.T) {
		var net1, net2 models.Network
		net1.NetID = "skylink"
		net1.AddressRange = "10.0.0.1/24"
		net1.DisplayName = "mynetwork"
		net2.NetID = "skylink"
		net2.AddressRange = "10.0.1.1/24"
		net2.IsDualStack = &no

		err := CreateNetwork(net1)
		assert.Nil(t, err)
		err = ValidateNetworkCreate(net2)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'NetID' failed on the 'netid_valid' tag")
	})
	t.Run("DuplicateDisplayName", func(t *testing.T) {
		var network models.Network
		network.NetID = "wirecat"
		network.AddressRange = "10.0.100.1/24"
		network.IsDualStack = &no
		network.DisplayName = "mynetwork"
		err := ValidateNetworkCreate(network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'DisplayName' failed on the 'displayname_unique' tag")
	})

}
