package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitl/netmaker/pro/idp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphgroups "github.com/microsoftgraph/msgraph-sdk-go/groups"
	msgraphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
	"os"
)

type Client struct {
	client *msgraphsdk.GraphServiceClient
}

func NewAzureEntraIDClient() (*Client, error) {
	tenantID := os.Getenv("AZURE_TENANT")
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
	}

	client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil
}

func (a *Client) GetUsers() ([]idp.User, error) {
	resp, err := a.client.Users().Get(context.TODO(), &msgraphusers.UsersRequestBuilderGetRequestConfiguration{
		QueryParameters: &msgraphusers.UsersRequestBuilderGetQueryParameters{
			Select: []string{"id", "userPrincipalName"},
		},
	})
	if err != nil {
		return nil, err
	}

	users := resp.GetValue()

	retval := make([]idp.User, len(users))
	for i, user := range users {
		retval[i] = idp.User{
			ID:       *user.GetId(),
			Username: *user.GetUserPrincipalName(),
		}
	}

	return retval, nil
}

func (a *Client) GetGroups() ([]idp.Group, error) {
	resp, err := a.client.Groups().Get(context.TODO(), &msgraphgroups.GroupsRequestBuilderGetRequestConfiguration{
		QueryParameters: &msgraphgroups.GroupsRequestBuilderGetQueryParameters{
			Select: []string{"id", "displayName"},
			Expand: []string{"members"},
		},
	})
	if err != nil {
		return nil, err
	}

	groups := resp.GetValue()

	retval := make([]idp.Group, len(groups))
	for i, group := range groups {
		members := group.GetMembers()

		retvalMembers := make([]string, len(members))
		for j, member := range members {
			retvalMembers[j] = *member.GetId()
		}

		retval[i] = idp.Group{
			ID:      *group.GetId(),
			Name:    *group.GetDisplayName(),
			Members: retvalMembers,
		}
	}

	return retval, nil
}
