package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/idp"
	"github.com/hashicorp/go-retryablehttp"
)

var linkNextRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

type Client struct {
	orgURL   string
	apiToken string
	client   *retryablehttp.Client
}

func NewOktaClient(oktaOrgURL, oktaAPIToken string) (*Client, error) {
	client := retryablehttp.NewClient()
	client.Logger = nil
	return &Client{
		orgURL:   strings.TrimRight(oktaOrgURL, "/"),
		apiToken: oktaAPIToken,
		client:   client,
	}, nil
}

func NewOktaClientFromSettings() (*Client, error) {
	settings := logic.GetServerSettings()
	return NewOktaClient(settings.OktaOrgURL, settings.OktaAPIToken)
}

type oktaUserProfile struct {
	Login     string  `json:"login"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}

type oktaUser struct {
	ID      string          `json:"id"`
	Status  string          `json:"status"`
	Profile oktaUserProfile `json:"profile"`
}

type oktaGroupProfile struct {
	Name string `json:"name"`
}

type oktaGroup struct {
	ID      string           `json:"id"`
	Profile oktaGroupProfile `json:"profile"`
}

type oktaGroupMember struct {
	ID string `json:"id"`
}

func (o *Client) get(ctx context.Context, rawURL string, out any) (nextURL string, err error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "SSWS "+o.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("okta returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return "", err
	}

	if link := resp.Header.Get("Link"); link != "" {
		if m := linkNextRe.FindStringSubmatch(link); len(m) > 1 {
			nextURL = m[1]
		}
	}
	return nextURL, nil
}

func (o *Client) Verify() error {
	var users []oktaUser
	if _, err := o.get(context.TODO(), o.orgURL+"/api/v1/users?limit=1", &users); err != nil {
		return err
	}
	var groups []oktaGroup
	_, err := o.get(context.TODO(), o.orgURL+"/api/v1/groups?limit=1", &groups)
	return err
}

func (o *Client) GetUsers(filters []string) ([]idp.User, error) {
	q := url.Values{"limit": {"200"}}
	if search := buildPrefixFilter("profile.login", filters); search != "" {
		q.Set("search", search)
	}

	nextURL := o.orgURL + "/api/v1/users?" + q.Encode()
	var retval []idp.User

	for nextURL != "" {
		var users []oktaUser
		var err error
		nextURL, err = o.get(context.TODO(), nextURL, &users)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			displayName := ""
			if u.Profile.FirstName != nil && u.Profile.LastName != nil {
				displayName = fmt.Sprintf("%s %s", *u.Profile.FirstName, *u.Profile.LastName)
			}
			retval = append(retval, idp.User{
				ID:              u.ID,
				Username:        u.Profile.Login,
				DisplayName:     displayName,
				AccountDisabled: u.Status == "SUSPENDED",
				AccountArchived: false,
			})
		}
	}
	return retval, nil
}

func (o *Client) GetGroups(filters []string) ([]idp.Group, error) {
	q := url.Values{}
	if search := buildPrefixFilter("profile.name", filters); search != "" {
		q.Set("search", search)
	}

	nextURL := o.orgURL + "/api/v1/groups?" + q.Encode()
	var retval []idp.Group

	for nextURL != "" {
		var groups []oktaGroup
		var err error
		nextURL, err = o.get(context.TODO(), nextURL, &groups)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			members, err := o.listGroupMembers(g.ID)
			if err != nil {
				return nil, err
			}
			retval = append(retval, idp.Group{
				ID:      g.ID,
				Name:    g.Profile.Name,
				Members: members,
			})
		}
	}
	return retval, nil
}

func (o *Client) listGroupMembers(groupID string) ([]string, error) {
	nextURL := fmt.Sprintf("%s/api/v1/groups/%s/users", o.orgURL, groupID)
	var members []string

	for nextURL != "" {
		var batch []oktaGroupMember
		var err error
		nextURL, err = o.get(context.TODO(), nextURL, &batch)
		if err != nil {
			return nil, err
		}
		for _, m := range batch {
			members = append(members, m.ID)
		}
	}
	return members, nil
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
