package schema

// ListModels lists all the models in this schema.
func ListModels() []interface{} {
	return []interface{}{
		&Job{},
		&Egress{},
		&UserAccessToken{},
		&Event{},
		&PendingHost{},
		&Nameserver{},
		&PostureCheck{},
		&User{},
		&Network{},
		&UserRole{},
		&UserGroup{},
		&JITRequest{},
		&JITGrant{},
		&Host{},
		&PendingUser{},
		&UserInvite{},
		&Gateway{},
		&Node{},
		&PostureCheckViolation{},
	}
}
