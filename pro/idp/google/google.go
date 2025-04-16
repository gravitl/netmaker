package google

import (
	"context"
	"github.com/gravitl/netmaker/pro/idp"
	admindir "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

type Client struct {
	service *admindir.Service
}

func NewGoogleWorkspaceClient() (*Client, error) {
	service, err := admindir.NewService(context.TODO(), option.WithCredentialsFile("credentials.json"))
	if err != nil {
		return nil, err
	}

	return &Client{
		service: service,
	}, nil
}

func (g *Client) GetUsers() ([]idp.User, error) {
	resp, err := g.service.Users.List().Fields("id", "primaryEmail", "suspended").Do()
	if err != nil {
		return nil, err
	}

	retval := make([]idp.User, len(resp.Users))
	for i, user := range resp.Users {
		retval[i] = idp.User{
			ID:              user.Id,
			Username:        user.PrimaryEmail,
			AccountDisabled: user.Suspended,
		}
	}

	return retval, nil
}

func (g *Client) GetGroups() ([]idp.Group, error) {
	resp, err := g.service.Groups.List().Fields("id", "name").Do()
	if err != nil {
		return nil, err
	}

	retval := make([]idp.Group, len(resp.Groups))
	for i, group := range resp.Groups {
		members, err := g.service.Members.List(group.Id).Fields("id").Do()
		if err != nil {
			return nil, err
		}

		retvalMembers := make([]string, len(members.Members))
		for j, member := range members.Members {
			retvalMembers[j] = member.Id
		}

		retval[i] = idp.Group{
			ID:      group.Id,
			Name:    group.Name,
			Members: retvalMembers,
		}
	}

	return retval, nil
}
