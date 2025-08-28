package okta

import (
	"context"
	"fmt"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/idp"
	"github.com/okta/okta-sdk-golang/v5/okta"
)

type Client struct {
	client *okta.APIClient
}

func NewOktaClient(oktaOrgURL, oktaAPIToken string) (*Client, error) {
	config, err := okta.NewConfiguration(
		okta.WithOrgUrl(oktaOrgURL),
		okta.WithToken(oktaAPIToken),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: okta.NewAPIClient(config),
	}, nil
}

func NewOktaClientFromSettings() (*Client, error) {
	settings := logic.GetServerSettings()

	return NewOktaClient(settings.OktaOrgURL, settings.OktaAPIToken)
}

func (o *Client) Verify() error {
	_, _, err := o.client.UserAPI.ListUsers(context.TODO()).Limit(1).Execute()
	if err != nil {
		return err
	}

	_, _, err = o.client.GroupAPI.ListGroups(context.TODO()).Limit(1).Execute()
	return err
}

func (o *Client) GetUsers(filters []string) ([]idp.User, error) {
	var retval []idp.User
	var allUsersFetched bool

	for !allUsersFetched {
		users, resp, err := o.client.UserAPI.ListUsers(context.TODO()).Execute()
		if err != nil {
			return nil, err
		}

		allUsersFetched = !resp.HasNextPage()

		for _, user := range users {
			id := *user.Id
			username := *user.Profile.Login

			displayName := ""
			if user.Profile.FirstName.IsSet() && user.Profile.LastName.IsSet() {
				displayName = fmt.Sprintf("%s %s", *user.Profile.FirstName.Get(), *user.Profile.LastName.Get())
			}

			accountDisabled := false
			if *user.Status == "SUSPENDED" {
				accountDisabled = true
			}

			retval = append(retval, idp.User{
				ID:              id,
				Username:        username,
				DisplayName:     displayName,
				AccountDisabled: accountDisabled,
				AccountArchived: false,
			})
		}
	}

	return retval, nil
}

func (o *Client) GetGroups(filters []string) ([]idp.Group, error) {
	var retval []idp.Group
	var allGroupsFetched bool

	for !allGroupsFetched {
		groups, resp, err := o.client.GroupAPI.ListGroups(context.TODO()).Execute()
		if err != nil {
			return nil, err
		}

		allGroupsFetched = !resp.HasNextPage()

		for _, group := range groups {
			var allMembersFetched bool
			id := *group.Id
			name := *group.Profile.Name

			var members []string
			for !allMembersFetched {
				groupUsers, resp, err := o.client.GroupAPI.ListGroupUsers(context.TODO(), id).Execute()
				if err != nil {
					return nil, err
				}

				allMembersFetched = !resp.HasNextPage()

				for _, groupUser := range groupUsers {
					members = append(members, *groupUser.Id)
				}
			}

			retval = append(retval, idp.Group{
				ID:      id,
				Name:    name,
				Members: members,
			})
		}
	}

	return retval, nil
}
