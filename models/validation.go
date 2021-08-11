package models

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

func CheckYesOrNo(fl validator.FieldLevel) bool {
	return fl.Field().String() == "yes" || fl.Field().String() == "no"
}

func CheckRegex(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(fl.Param())
	return re.MatchString(fl.Field().String())
}
