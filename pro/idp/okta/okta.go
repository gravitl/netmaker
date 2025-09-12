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
		okta.WithRateLimitPrevent(true),
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

	users, resp, err := o.client.UserAPI.ListUsers(context.TODO()).
		Search(buildPrefixFilter("profile.login", filters)).
		Execute()
	if err != nil {
		return nil, err
	}

	usersProcessingPending := len(users) > 0 || resp.HasNextPage()

	for usersProcessingPending {
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

		if resp.HasNextPage() {
			users = make([]okta.User, 0)

			resp, err = resp.Next(&users)
			if err != nil {
				return nil, err
			}

			usersProcessingPending = len(users) > 0 || resp.HasNextPage()
		} else {
			usersProcessingPending = false
		}
	}

	return retval, nil
}

func (o *Client) GetGroups(filters []string) ([]idp.Group, error) {
	var retval []idp.Group

	groups, resp, err := o.client.GroupAPI.ListGroups(context.TODO()).
		Search(buildPrefixFilter("profile.name", filters)).
		Execute()
	if err != nil {
		return nil, err
	}

	groupsProcessingPending := len(groups) > 0 || resp.HasNextPage()

	for groupsProcessingPending {
		for _, group := range groups {
			id := *group.Id
			name := *group.Profile.Name

			var members []string
			groupUsers, groupUsersResp, err := o.client.GroupAPI.ListGroupUsers(context.TODO(), id).Execute()
			if err != nil {
				return nil, err
			}

			groupUsersProcessingPending := len(groupUsers) > 0 || groupUsersResp.HasNextPage()

			for groupUsersProcessingPending {
				for _, groupUser := range groupUsers {
					members = append(members, *groupUser.Id)
				}

				if groupUsersResp.HasNextPage() {
					groupUsers = make([]okta.GroupMember, 0)

					groupUsersResp, err = groupUsersResp.Next(&groupUsers)
					if err != nil {
						return nil, err
					}

					groupUsersProcessingPending = len(groupUsers) > 0 || groupUsersResp.HasNextPage()
				} else {
					groupUsersProcessingPending = false
				}
			}

			retval = append(retval, idp.Group{
				ID:      id,
				Name:    name,
				Members: members,
			})
		}

		if resp.HasNextPage() {
			groups = make([]okta.Group, 0)

			resp, err = resp.Next(&groups)
			if err != nil {
				return nil, err
			}

			groupsProcessingPending = len(groups) > 0 || resp.HasNextPage()
		} else {
			groupsProcessingPending = false
		}
	}

	return retval, nil
}

func buildPrefixFilter(field string, prefixes []string) string {
	if len(prefixes) == 0 {
		return ""
	}

	if len(prefixes) == 1 {
		return fmt.Sprintf("%s sw \"%s\"", field, prefixes[0])
	}

	return buildPrefixFilter(field, prefixes[:1]) + " or " + buildPrefixFilter(field, prefixes[1:])
}
