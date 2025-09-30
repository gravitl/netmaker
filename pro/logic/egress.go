package logic

import (
	"errors"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
)

func ValidateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	_, err := logic.GetNetwork(e.Network)
	if err != nil {
		return errors.New("failed to get network " + err.Error())
	}

	if !servercfg.IsPro && len(e.Nodes) > 1 {
		return errors.New("can only set one routing node on CE")
	}

	if len(e.Nodes) > 0 {
		for k := range e.Nodes {
			_, err := logic.GetNodeByID(k)
			if err != nil {
				return errors.New("invalid routing node " + err.Error())
			}
		}
	}
	e.Tags = make(datatypes.JSONMap)
	if len(e.Tags) > 0 {
		e.Nodes = make(datatypes.JSONMap)
		for tagID := range e.Tags {
			_, err := GetTag(models.TagID(tagID))
			if err != nil {
				return errors.New("invalid tag " + tagID)
			}
		}
	}
	return nil
}
