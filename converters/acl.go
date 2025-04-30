package converters

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ToSchemaACL(acl models.Acl) schema.ACL {
	var src, dst []schema.PolicyGroupTag

	if len(acl.Src) > 0 {
		src = make([]schema.PolicyGroupTag, len(acl.Src))
		for i := range acl.Src {
			src[i] = schema.PolicyGroupTag{
				GroupType: string(acl.Src[i].ID),
				Tag:       acl.Src[i].Value,
			}
		}
	}

	if len(acl.Dst) > 0 {
		dst = make([]schema.PolicyGroupTag, len(acl.Dst))
		for i := range acl.Dst {
			dst[i] = schema.PolicyGroupTag{
				GroupType: string(acl.Dst[i].ID),
				Tag:       acl.Dst[i].Value,
			}
		}
	}

	return schema.ACL{
		ID:               acl.ID,
		NetworkID:        string(acl.NetworkID),
		Name:             acl.Name,
		MetaData:         acl.MetaData,
		Default:          acl.Default,
		Enabled:          acl.Enabled,
		PolicyType:       string(acl.RuleType),
		ServiceType:      acl.ServiceType,
		AllowedDirection: int(acl.AllowedDirection),
		Src:              src,
		Dst:              dst,
		Protocol:         string(acl.Proto),
		Port:             acl.Port,
		CreatedBy:        acl.CreatedBy,
		CreatedAt:        acl.CreatedAt,
	}
}

func ToSchemaACLs(acls []models.Acl) []schema.ACL {
	_acls := make([]schema.ACL, len(acls))
	for i, acl := range acls {
		_acls[i] = ToSchemaACL(acl)
	}

	return _acls
}

func ToModelACL(_acl schema.ACL) models.Acl {
	var src, dst []models.AclPolicyTag

	if len(_acl.Src) > 0 {
		src = make([]models.AclPolicyTag, len(_acl.Src))
		for i := range _acl.Src {
			src[i] = models.AclPolicyTag{
				ID:    models.AclGroupType(_acl.Src[i].GroupType),
				Value: _acl.Src[i].Tag,
			}
		}
	}

	if len(_acl.Dst) > 0 {
		dst = make([]models.AclPolicyTag, len(_acl.Dst))
		for i := range _acl.Dst {
			dst[i] = models.AclPolicyTag{
				ID:    models.AclGroupType(_acl.Dst[i].GroupType),
				Value: _acl.Dst[i].Tag,
			}
		}
	}

	return models.Acl{
		ID:               _acl.ID,
		Default:          _acl.Default,
		MetaData:         _acl.MetaData,
		Name:             _acl.Name,
		NetworkID:        models.NetworkID(_acl.NetworkID),
		RuleType:         models.AclPolicyType(_acl.PolicyType),
		Src:              src,
		Dst:              dst,
		Proto:            models.Protocol(_acl.Protocol),
		ServiceType:      _acl.ServiceType,
		Port:             _acl.Port,
		AllowedDirection: models.AllowedTrafficDirection(_acl.AllowedDirection),
		Enabled:          _acl.Enabled,
		CreatedBy:        _acl.CreatedBy,
		CreatedAt:        _acl.CreatedAt,
	}
}

func ToModelACLs(_acls []schema.ACL) []models.Acl {
	acls := make([]models.Acl, len(_acls))
	for i, _acl := range _acls {
		acls[i] = ToModelACL(_acl)
	}

	return acls
}
