package schema

import "github.com/gravitl/netmaker/models"

// ListModels lists all the models in this schema.
func ListModels() []interface{} {
	return []interface{}{
		&Job{},
		&models.UserAccessToken{},
	}
}
