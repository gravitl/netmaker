package promodels

type Void struct{}

// UserGroupName - string representing a group name
type UserGroupName string

// UserGroups - groups type, holds group names
type UserGroups map[UserGroupName]Void
