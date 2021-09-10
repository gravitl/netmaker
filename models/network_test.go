package models

// moved from controllers need work
//func TestUpdateNetwork(t *testing.T) {
//	database.InitializeDatabase()
//	createNet()
//	network := getNet()
//	t.Run("NetID", func(t *testing.T) {
//		var networkupdate models.Network
//		networkupdate.NetID = "wirecat"
//		Range, local, err := network.Update(&networkupdate)
//		assert.NotNil(t, err)
//		assert.Equal(t, "NetID is not editable", err.Error())
//		t.Log(err, Range, local)
//	})
//	t.Run("LocalRange", func(t *testing.T) {
//		var networkupdate models.Network
//		//NetID needs to be set as it will be in updateNetwork
//		networkupdate.NetID = "skynet"
//		networkupdate.LocalRange = "192.168.0.1/24"
//		Range, local, err := network.Update(&networkupdate)
//		assert.Nil(t, err)
//		t.Log(err, Range, local)
//	})
//}
