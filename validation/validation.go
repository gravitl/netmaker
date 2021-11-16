package validation

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// CheckYesOrNo - checks if a field on a struct is yes or no
func CheckYesOrNo(fl validator.FieldLevel) bool {
	return fl.Field().String() == "yes" || fl.Field().String() == "no"
}

// CheckRegex - check if a struct's field passes regex test
func CheckRegex(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(fl.Param())
	return re.MatchString(fl.Field().String())
}
