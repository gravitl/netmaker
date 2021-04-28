package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	mongoconn.ConnectDatabase()
	//var waitgroup sync.WaitGroup
	//waitgroup.Add(1)
	//go controller.HandleRESTRequests(&waitgroup)
	var gconf models.GlobalConfig
	gconf.ServerGRPC = "localhost:8081"
	gconf.PortGRPC = "50051"
	//err := SetGlobalConfig(gconf)
	collection := mongoconn.Client.Database("netmaker").Collection("config")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	//create, _, err := functions.GetGlobalConfig()
	_, err := collection.InsertOne(ctx, gconf)
	if err != nil {
		panic("could not create config store")
	}

	//wait for http server to start
	//time.Sleep(time.Second * 1)
	os.Exit(m.Run())
}

func TestDeleteUser(t *testing.T) {
	hasadmin, err := HasAdmin()
	assert.Nil(t, err, err)
	if !hasadmin {
		//if !adminExists() {
		user := models.User{"admin", "admin", true}
		_, err := CreateUser(user)
		assert.Nil(t, err, err)
	}
	t.Run("ExistingUser", func(t *testing.T) {
		deleted, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		assert.True(t, deleted)
	})
	t.Run("NonExistantUser", func(t *testing.T) {
		deleted, err := DeleteUser("admin")
		assert.Nil(t, err, err)
		assert.False(t, deleted)
	})
}
