package api

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type RequestValidator struct {
	validator *validator.Validate
}

func initRequestValidator() *RequestValidator {
	requestValidator := validator.New(validator.WithRequiredStructEnabled())
	// Prefer the query tag for parsed parameters over the struct field name
	tagNameFunc := func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("query"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	}
	requestValidator.RegisterTagNameFunc(tagNameFunc)
	return &RequestValidator{validator: requestValidator}
}

func (v RequestValidator) Validate(data any) error {
	err := v.validator.Struct(data)
	if err != nil {
		validationErrors := []string{}
		for _, err := range err.(validator.ValidationErrors) {
			switch err.Tag() {
			case "required":
				validationErrors = append(validationErrors, fmt.Sprintf("%s is required", err.Field()))
			default:
				validationErrors = append(validationErrors, fmt.Sprintf("%s is invalid", err.Field()))
			}
		}
		return fiber.NewError(fiber.StatusBadRequest, strings.Join(validationErrors, "; "))
	}
	return nil
}
