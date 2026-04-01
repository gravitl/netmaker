package serverctl

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

const (
	// NETMAKER_BINARY_NAME - name of netmaker binary
	NETMAKER_BINARY_NAME = "netmaker"
)

func SetDefaults() error {
	if err := setNodeDefaults(); err != nil {
		return err
	}

	if err := setUserDefaults(); err != nil {
		return err
	}

	return nil
}

// setNodeDefaults - runs through each node and set defaults
func setNodeDefaults() error {
	// upgraded systems will not have ACL's set, which is why we need this function
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	for i := range nodes {
		logic.SetNodeDefaults(&nodes[i], false)
		logic.UpsertNode(&nodes[i])
	}
	return nil
}

func setUserDefaults() error {
	users, err := logic.GetUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		updateUser := &schema.User{Username: user.UserName}
		err = updateUser.Get(db.WithContext(context.TODO()))
		if err != nil {
			slog.Error("could not get user", "user", updateUser.Username, "error", err.Error())
		}
		logic.SetUserDefaults(updateUser)
		err = logic.UpsertUser(*updateUser)
		if err != nil {
			slog.Error("could not update user", "user", updateUser.Username, "error", err.Error())
		}
	}
	return nil
}
