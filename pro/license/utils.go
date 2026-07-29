package license

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

// base64encode - base64 encode helper function
func base64encode(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// base64decode - base64 decode helper function
func base64decode(input string) []byte {
	bytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil
	}

	return bytes
}

func tenantLimits(ctx context.Context) (Limits, bool) {
	if scope.Level(ctx) != scope.TenantScope {
		return Limits{}, false
	}

	response, _ := getCachedResponse(ctx)
	tenantID := scope.ID(ctx)
	for _, t := range response.Tenants {
		if t.ID == tenantID {
			return t.Limits, true
		}
	}
	return Limits{}, false
}

func limitExceeded(ctx context.Context, limit int, count func() (int, error)) bool {
	if !EnforceLimits(ctx) {
		return false
	}
	if limit <= 0 {
		return false
	}
	usage, err := count()
	if err != nil {
		return false
	}
	return usage >= limit
}

func HostLimitExceeded(ctx context.Context) bool {
	limits, ok := tenantLimits(ctx)
	if !ok {
		return false
	}
	return limitExceeded(ctx, limits.Hosts, func() (int, error) {
		return (&schema.Host{}).Count(ctx)
	})
}

func NetworkLimitExceeded(ctx context.Context) bool {
	limits, ok := tenantLimits(ctx)
	if !ok {
		return false
	}
	return limitExceeded(ctx, limits.Networks, func() (int, error) {
		return (&schema.Network{}).Count(ctx)
	})
}

func ClientLimitExceeded(ctx context.Context) bool {
	limits, ok := tenantLimits(ctx)
	if !ok {
		return false
	}
	return limitExceeded(ctx, limits.Clients, func() (int, error) {
		return (&schema.ExtClientRecord{}).Count(ctx)
	})
}

func UserLimitExceeded(ctx context.Context) bool {
	limits, ok := tenantLimits(ctx)
	if !ok {
		return false
	}
	return limitExceeded(ctx, limits.Users, func() (int, error) {
		return (&schema.User{}).CountWithMembership(ctx)
	})
}

func IngressLimitExceeded(ctx context.Context) bool {
	limits, ok := tenantLimits(ctx)
	if !ok {
		return false
	}
	return limitExceeded(ctx, limits.Ingresses, func() (int, error) {
		return (&schema.Node{}).Count(ctx, dbtypes.WithFilter("is_gateway", true))
	})
}

func EgressLimitExceeded(ctx context.Context) bool {
	limits, ok := tenantLimits(ctx)
	if !ok {
		return false
	}
	return limitExceeded(ctx, limits.Egresses, func() (int, error) {
		return (&schema.Egress{}).Count(ctx)
	})
}

func GetFeatureFlags(ctx context.Context) models.FeatureFlags {
	response, _ := getCachedResponse(ctx)

	if scope.Level(ctx) == scope.TenantScope {
		tenantID := scope.ID(ctx)
		for _, t := range response.Tenants {
			if t.ID == tenantID {
				return t.FeatureFlags
			}
		}
	}

	return response.FeatureFlags
}

func EnforceLimits(ctx context.Context) bool {
	response, _ := getCachedResponse(ctx)
	if scope.Level(ctx) == scope.TenantScope {
		tenantID := scope.ID(ctx)
		for _, t := range response.Tenants {
			if t.ID == tenantID {
				return t.EnforceLimits
			}
		}
	}

	return false
}

func IsMSP(ctx context.Context) bool {
	response, err := getCachedResponse(ctx)
	if err != nil {
		return false
	}

	return response.Organization.ID != ""
}

func ErrLicenseValidation(ctx context.Context) error {
	invalidErr := licenseInvalidErr.Load()
	if invalidErr != nil {
		return *invalidErr
	}

	response, err := getCachedResponse(ctx)
	if err != nil {
		return err
	}

	if scope.Level(ctx) != scope.TenantScope {
		return nil
	}

	tenantID := scope.ID(ctx)
	for _, t := range response.Tenants {
		if t.ID == tenantID {
			return tenantStatusError(t.Status)
		}
	}

	return nil
}

func tenantStatusError(status TenantStatusMessage) error {
	switch status {
	case "", TenantStatusOk:
		return nil
	case TenantStatusPaymentInvalid:
		return errors.New("invalid payment method on file")
	case TenantStatusPaymentMissing:
		return errors.New("no payment method on file")
	case TenantStatusLicenseExpired:
		return errors.New("license has expired")
	default:
		return fmt.Errorf("tenant status: %s", status)
	}
}
