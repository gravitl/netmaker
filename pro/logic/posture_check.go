package logic

import (
	"errors"

	"github.com/biter777/countries"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func ValidatePostureCheck(pc *schema.PostureCheck) error {
	if pc.Name == "" {
		return errors.New("name cannot be empty")
	}
	_, err := logic.GetNetwork(pc.NetworkID)
	if err != nil {
		return errors.New("invalid network")
	}
	if pc.Attribute != schema.AutoUpdate && pc.Attribute != schema.OS && pc.Attribute != schema.OSVersion &&
		pc.Attribute != schema.ClientLocation &&
		pc.Attribute != schema.ClientVersion {
		return errors.New("unkown attribute")
	}
	if len(pc.Values) == 0 {
		return errors.New("attribute value cannot be empty")
	}
	if len(pc.Tags) > 0 {
		for tagID := range pc.Tags {
			if tagID == "*" {
				continue
			}
			_, err := GetTag(models.TagID(tagID))
			if err != nil {
				return errors.New("unknown tag")
			}
		}
	} else {
		pc.Tags = make(datatypes.JSONMap)
		pc.Tags["*"] = struct{}{}
	}
	if pc.Attribute == schema.ClientLocation {
		for _, loc := range pc.Values {
			if countries.ByName(loc) == countries.Unknown {
				return errors.New("invalid country code")
			}
		}
	}
	return nil
}
