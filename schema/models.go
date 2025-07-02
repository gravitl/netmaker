package schema

// ListModels lists all the models in this schema.
func ListModels() []interface{} {
	return []interface{}{
		&Host{},
		&Network{},
		&Node{},
		&ACL{},
		&Job{},
		&Egress{},
		&UserAccessToken{},
		&Event{},
	}
}
