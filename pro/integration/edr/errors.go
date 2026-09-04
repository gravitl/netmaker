package edr

import "errors"

var (
	ErrDeviceNotFoundInEDR = errors.New("device_not_found_in_edr")
)

func LookupErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrDeviceNotFoundInEDR) {
		return ErrDeviceNotFoundInEDR.Error()
	}
	return ""
}
