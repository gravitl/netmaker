package models

type UserGroup struct {
	ID                 string                     `json:"id"`
	PermissionTemplate UserRolePermissionTemplate `json:"role_permission_template"`
	MetaData           string                     `json:"meta_data"`
}
