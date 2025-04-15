package google

import "github.com/gravitl/netmaker/pro/idp"

type Client struct{}

func NewGoogleWorkspaceClient() (*Client, error) {
	return &Client{}, nil
}

func (g *Client) GetUsers() ([]idp.User, error) {
	//TODO implement me
	panic("implement me")
}

func (g *Client) GetGroups() ([]idp.Group, error) {
	//TODO implement me
	panic("implement me")
}
