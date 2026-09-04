package license

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/gravitl/netmaker/schema"
)

func hasCachedResponse(ctx context.Context) bool {
	cached := &schema.Internal{Key: schema.InternalKey_LicenseValidationCachedResponse}
	return cached.Get(ctx) == nil
}

var cachedResponse atomic.Pointer[ValidatedLicense]

func cacheResponse(ctx context.Context, response []byte) error {
	var validationResponse ValidatedLicense
	err := json.Unmarshal(response, &validationResponse)
	if err != nil {
		return err
	}

	cachedResponse.Store(&validationResponse)
	return cacheResponseInDB(ctx, response)
}

var errNoCachedResponse = errors.New("no cached license validation response available")

func getCachedResponse(ctx context.Context) (ValidatedLicense, error) {
	response := cachedResponse.Load()
	if response == nil {
		raw, err := getCachedResponseFromDB(ctx)
		if err != nil {
			return ValidatedLicense{}, errNoCachedResponse
		}

		var validationResponse ValidatedLicense
		err = json.Unmarshal(raw, &validationResponse)
		if err != nil {
			return ValidatedLicense{}, errNoCachedResponse
		}

		cachedResponse.Store(&validationResponse)
		return validationResponse, nil
	}

	return *response, nil
}

func clearCachedResponse(ctx context.Context) error {
	cachedResponse.Store(nil)
	return clearCachedResponseFromDB(ctx)
}

func cacheResponseInDB(ctx context.Context, response []byte) error {
	cached := &schema.Internal{
		Key:   schema.InternalKey_LicenseValidationCachedResponse,
		Value: base64encode(response),
	}
	return cached.Set(ctx)
}

func getCachedResponseFromDB(ctx context.Context) ([]byte, error) {
	cached := &schema.Internal{
		Key: schema.InternalKey_LicenseValidationCachedResponse,
	}
	err := cached.Get(ctx)
	if err != nil {
		return nil, err
	}

	return base64decode(cached.Value), nil
}

func clearCachedResponseFromDB(ctx context.Context) error {
	cached := &schema.Internal{
		Key:   schema.InternalKey_LicenseValidationCachedResponse,
		Value: base64encode([]byte("{}")),
	}
	return cached.Set(ctx)
}
