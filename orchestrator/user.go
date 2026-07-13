package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/gravitl/netmaker/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UserOrchestrator struct {
	userExt extensions.UserExtensions
}

func (u *UserOrchestrator) CreateUser(ctx context.Context, user *schema.User, opts ...Option) error {
	ops := applyOptions(opts...)

	existing := &schema.User{
		Username: user.Username,
	}
	err := existing.Get(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		err = user.Create(ctx)
		if err != nil {
			return err
		}
	} else {
		user.ID = existing.ID
	}

	if user.UserGroups.Data() == nil {
		user.UserGroups = datatypes.NewJSONType(map[schema.UserGroupID]struct{}{})
	}

	if ops.inheritedAuth {
		user.AuthType = schema.Inherited
		user.Password = ""
		user.ExternalIdentityProviderID = ""
	} else {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
		if err != nil {
			return err
		}

		user.Password = string(passwordHash)

		err = u.userExt.ConfigureAuthType(user)
		if err != nil {
			return err
		}
	}

	if scope.Level(ctx) == scope.TenantScope {
		tenantID := scope.ID(ctx)
		membership := &schema.TenantMembership{
			TenantID:                   tenantID,
			UserID:                     user.ID,
			RoleID:                     user.PlatformRoleID,
			AuthType:                   user.AuthType,
			ExternalIdentityProviderID: user.ExternalIdentityProviderID,
			Password:                   user.Password,
		}

		u.userExt.ConfigureGroups(membership, user.UserGroups)
		u.userExt.ConfigureGlobalAdminGroup(membership)

		err = membership.Create(ctx)
		if err != nil {
			return err
		}
	} else if scope.Level(ctx) == scope.OrgScope {
		orgID := scope.ID(ctx)
		membership := &schema.OrgMembership{
			OrganizationID:             orgID,
			UserID:                     user.ID,
			RoleID:                     user.PlatformRoleID,
			AuthType:                   user.AuthType,
			ExternalIdentityProviderID: user.ExternalIdentityProviderID,
			Password:                   user.Password,
		}
		err = membership.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u *UserOrchestrator) ValidateCreateUser(ctx context.Context, user *schema.User, _ ...Option) error {
	var userExists bool
	existing := &schema.User{
		Username: user.Username,
	}
	err := existing.Get(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			userExists = false
		} else {
			return err
		}
	} else {
		userExists = true
	}

	var validationErr error
	err = u.validateUsername(user.Username)
	if err != nil {
		validationErr = errors.Join(validationErr, err)
	}

	if len(user.Password) < 5 {
		validationErr = errors.Join(validationErr, errors.New("password must have a minimum of 5 characters"))
	}

	if scope.Level(ctx) == scope.TenantScope {
		tenant := &schema.Tenant{
			ID: scope.ID(ctx),
		}
		err = tenant.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				validationErr = errors.Join(validationErr, fmt.Errorf("tenant %s does not exist", tenant.ID))
			} else {
				return err
			}
		} else {
			validTenantRoles := map[schema.UserRoleID]bool{
				schema.SuperAdminRole: true,
				schema.AdminRole:      true,
				schema.PlatformUser:   true,
				schema.ServiceUser:    true,
				schema.Auditor:        true,
			}

			_, ok := validTenantRoles[user.PlatformRoleID]
			if !ok {
				validationErr = errors.Join(validationErr, fmt.Errorf("invalid user role: %s", user.PlatformRoleID))
			}

			if userExists {
				membership := &schema.TenantMembership{
					TenantID: tenant.ID,
					UserID:   existing.ID,
				}
				err = membership.Get(ctx)
				if err != nil {
					return err
				}

				validationErr = errors.Join(validationErr, fmt.Errorf("user %s already exists in tenant %s", user.Username, tenant.ID))
			}
		}
	} else if scope.Level(ctx) == scope.OrgScope {
		org := &schema.Organization{
			ID: scope.ID(ctx),
		}
		err = org.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				validationErr = errors.Join(validationErr, fmt.Errorf("organization %s does not exist", org.ID))
			} else {
				return err
			}
		} else {
			validOrganizationRoles := map[schema.UserRoleID]bool{
				schema.OrgOwner: true,
				schema.OrgAdmin: true,
			}

			_, ok := validOrganizationRoles[user.PlatformRoleID]
			if !ok {
				validationErr = errors.Join(validationErr, fmt.Errorf("invalid user role: %s", user.PlatformRoleID))
			}

			if userExists {
				membership := &schema.OrgMembership{
					OrganizationID: org.ID,
					UserID:         existing.ID,
				}
				err = membership.Get(ctx)
				if err != nil {
					return err
				}

				validationErr = errors.Join(validationErr, fmt.Errorf("user %s already exists in organization %s", user.Username, org.ID))
			}
		}
	}

	return validationErr
}

func (u *UserOrchestrator) validateUsername(username string) error {
	var validationErr error

	if len(username) == 0 {
		validationErr = errors.Join(validationErr, errors.New("username cannot be empty"))
	} else if len(username) <= 3 {
		validationErr = errors.Join(validationErr, errors.New("username must have more than 3 characters"))
	}

	var isValidEmail bool
	_, err := mail.ParseAddress(username)
	if err == nil {
		isValidEmail = true
	}

	if !isValidEmail {
		charset := "abcdefghijklmnopqrstuvwxyz1234567890-."
		for _, char := range username {
			if !strings.Contains(charset, strings.ToLower(string(char))) {
				validationErr = errors.Join(validationErr, errors.New("invalid character(s) in username"))
				break
			}
		}
	}

	return validationErr
}
