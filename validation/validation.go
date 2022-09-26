package validation

import (
	"regexp"

	validator "github.com/go-playground/validator/v10"
)

// CheckYesOrNo - checks if a field on a struct is yes or no
func CheckYesOrNo(fl validator.FieldLevel) bool {
	return fl.Field().String() == "yes" || fl.Field().String() == "no"
}

// CheckYesOrNoOrUnset - checks if a field is yes, no or unset
func CheckYesOrNoOrUnset(fl validator.FieldLevel) bool {
	return CheckYesOrNo(fl) || fl.Field().String() == "unset"
}

// CheckRegex - check if a struct's field passes regex test
func CheckRegex(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(fl.Param())
	return re.MatchString(fl.Field().String())
}
