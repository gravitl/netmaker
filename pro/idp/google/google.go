package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/idp"
	admindir "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

type Client struct {
	service *admindir.Service
}

func NewGoogleWorkspaceClient() (*Client, error) {
	settings := logic.GetServerSettings()

	credsJson, err := base64.StdEncoding.DecodeString(settings.GoogleSACredsJson)
	if err != nil {
		return nil, err
	}

	credsJsonMap := make(map[string]interface{})
	err = json.Unmarshal(credsJson, &credsJsonMap)
	if err != nil {
		return nil, err
	}

	source, err := impersonate.CredentialsTokenSource(
		context.TODO(),
		impersonate.CredentialsConfig{
			TargetPrincipal: credsJsonMap["client_email"].(string),
			Scopes: []string{
				admindir.AdminDirectoryUserReadonlyScope,
				admindir.AdminDirectoryGroupReadonlyScope,
				admindir.AdminDirectoryGroupMemberReadonlyScope,
			},
			Subject: settings.GoogleAdminEmail,
		},
		option.WithCredentialsJSON(credsJson),
	)
	if err != nil {
		return nil, err
	}

	service, err := admindir.NewService(
		context.TODO(),
		option.WithTokenSource(source),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		service: service,
	}, nil
}

func (g *Client) GetUsers() ([]idp.User, error) {
	var retval []idp.User
	err := g.service.Users.List().
		Customer("my_customer").
		Fields("users(id,primaryEmail,name,suspended)", "nextPageToken").
		Pages(context.TODO(), func(users *admindir.Users) error {
			for _, user := range users.Users {
				retval = append(retval, idp.User{
					ID:              user.Id,
					Username:        user.PrimaryEmail,
					DisplayName:     user.Name.FullName,
					AccountDisabled: user.Suspended,
				})
			}

			return nil
		})

	return retval, err
}

func (g *Client) GetGroups() ([]idp.Group, error) {
	var retval []idp.Group
	err := g.service.Groups.List().
		Customer("my_customer").
		Fields("groups(id,name)", "nextPageToken").
		Pages(context.TODO(), func(groups *admindir.Groups) error {
			for _, group := range groups.Groups {
				var retvalMembers []string
				err := g.service.Members.List(group.Id).
					Fields("members(id)", "nextPageToken").
					Pages(context.TODO(), func(members *admindir.Members) error {
						for _, member := range members.Members {
							retvalMembers = append(retvalMembers, member.Id)
						}

						return nil
					})
				if err != nil {
					return err
				}

				retval = append(retval, idp.Group{
					ID:      group.Id,
					Name:    group.Name,
					Members: retvalMembers,
				})
			}

			return nil
		})

	return retval, err
}
