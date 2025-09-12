package idp

type Client interface {
	Verify() error
	GetUsers(filters []string) ([]User, error)
	GetGroups(filters []string) ([]Group, error)
}

type User struct {
	ID              string
	Username        string
	DisplayName     string
	AccountDisabled bool
	AccountArchived bool
}

type Group struct {
	ID      string
	Name    string
	Members []string
}
