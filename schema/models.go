package schema

// ListModels lists all the models in this schema.
func ListModels() []interface{} {
	return []interface{}{
		&Job{},
	}
}
