package mdm

import "errors"

// Posture-facing error codes returned by Entra-keyed MDM lookups.
var (
	ErrDeviceNotRegisteredInEntra = errors.New("device_not_registered_in_entra")
	ErrDeviceNotEnrolledInIntune  = errors.New("device_not_enrolled_in_intune")
	ErrDeviceNotFoundInMDM        = errors.New("device_not_found_in_mdm")
)

// LookupErrorCode maps a lookup error to a stable posture violation code.
// Returns "" for non-lookup failures (e.g. network errors).
func LookupErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrDeviceNotRegisteredInEntra) {
		return ErrDeviceNotRegisteredInEntra.Error()
	}
	if errors.Is(err, ErrDeviceNotEnrolledInIntune) {
		return ErrDeviceNotEnrolledInIntune.Error()
	}
	if errors.Is(err, ErrDeviceNotFoundInMDM) {
		return ErrDeviceNotFoundInMDM.Error()
	}
	return ""
}
