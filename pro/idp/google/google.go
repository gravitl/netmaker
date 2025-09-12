package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"net/url"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/idp"
	admindir "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

type Client struct {
	service *admindir.Service
}

func NewGoogleWorkspaceClient(adminEmail, creds string) (*Client, error) {
	credsJson, err := base64.StdEncoding.DecodeString(creds)
	if err != nil {
		return nil, err
	}

	credsJsonMap := make(map[string]interface{})
	err = json.Unmarshal(credsJson, &credsJsonMap)
	if err != nil {
		return nil, err
	}

	var targetPrincipal string
	_, ok := credsJsonMap["client_email"]
	if !ok {
		return nil, errors.New("invalid service account credentials: missing client_email field")
	} else {
		targetPrincipal = credsJsonMap["client_email"].(string)
	}

	source, err := impersonate.CredentialsTokenSource(
		context.TODO(),
		impersonate.CredentialsConfig{
			TargetPrincipal: targetPrincipal,
			Scopes: []string{
				admindir.AdminDirectoryUserReadonlyScope,
				admindir.AdminDirectoryGroupReadonlyScope,
				admindir.AdminDirectoryGroupMemberReadonlyScope,
			},
			Subject: adminEmail,
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

func NewGoogleWorkspaceClientFromSettings() (*Client, error) {
	settings := logic.GetServerSettings()

	return NewGoogleWorkspaceClient(settings.GoogleAdminEmail, settings.GoogleSACredsJson)
}

func (g *Client) Verify() error {
	_, err := g.service.Users.List().
		Customer("my_customer").
		MaxResults(1).
		Do()
	if err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) {
			return errors.New(gerr.Message)
		}

		var uerr *url.Error
		if errors.As(err, &uerr) {
			errMsg := strings.TrimSpace(uerr.Err.Error())
			if strings.Contains(errMsg, "{") && strings.HasSuffix(errMsg, "}") {
				// probably contains response json.
				_, jsonBody, _ := strings.Cut(errMsg, "{")
				jsonBody = "{" + jsonBody

				var errResp errorResponse
				err := json.Unmarshal([]byte(jsonBody), &errResp)
				if err == nil && errResp.Error.Message != "" {
					return errors.New(errResp.Error.Message)
				}
			}
		}

		return err
	}

	_, err = g.service.Groups.List().
		Customer("my_customer").
		MaxResults(1).
		Do()
	if err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) {
			return errors.New(gerr.Message)
		}

		return err
	}

	return nil
}

func (g *Client) GetUsers(filters []string) ([]idp.User, error) {
	var retval []idp.User
	err := g.service.Users.List().
		Customer("my_customer").
		Fields("users(id,primaryEmail,name,suspended,archived)", "nextPageToken").
		Pages(context.TODO(), func(users *admindir.Users) error {
			for _, user := range users.Users {
				var keep bool
				if len(filters) > 0 {
					for _, filter := range filters {
						if strings.HasPrefix(user.PrimaryEmail, filter) {
							keep = true
						}
					}
				} else {
					keep = true
				}

				if !keep {
					continue
				}

				retval = append(retval, idp.User{
					ID:              user.Id,
					Username:        user.PrimaryEmail,
					DisplayName:     user.Name.FullName,
					AccountDisabled: user.Suspended,
					AccountArchived: user.Archived,
				})
			}

			return nil
		})

	return retval, err
}

func (g *Client) GetGroups(filters []string) ([]idp.Group, error) {
	var retval []idp.Group
	err := g.service.Groups.List().
		Customer("my_customer").
		Fields("groups(id,name)", "nextPageToken").
		Pages(context.TODO(), func(groups *admindir.Groups) error {
			for _, group := range groups.Groups {
				var keep bool
				if len(filters) > 0 {
					for _, filter := range filters {
						if strings.HasPrefix(group.Name, filter) {
							keep = true
						}
					}
				} else {
					keep = true
				}

				if !keep {
					continue
				}

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

type errorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}
