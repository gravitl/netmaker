package idp

type Client interface {
	GetUsers() ([]User, error)
	GetGroups() ([]Group, error)
}

type User struct {
	ID              string
	Username        string
	DisplayName     string
	AccountDisabled bool
}

type Group struct {
	ID      string
	Name    string
	Members []string
}
