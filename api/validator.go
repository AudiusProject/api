package api

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gagliardetto/solana-go"
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
	requestValidator.RegisterValidation("coin_ticker", isCoinTicker)
	requestValidator.RegisterValidation("solana_address", isSolanaAddress)
	requestValidator.RegisterValidation("unicode_name", isUnicodeName)
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

const (
	coinTickerRegexString  = "^\\$[a-zA-Z0-9]+$"
	unicodeNameRegexString = "^[\\p{L}\\p{N} _-]+$"
)

var (
	coinTickerRegex  = lazyRegexCompile(coinTickerRegexString)
	unicodeNameRegex = lazyRegexCompile(unicodeNameRegexString)
)

func lazyRegexCompile(str string) func() *regexp.Regexp {
	var regex *regexp.Regexp
	var once sync.Once
	return func() *regexp.Regexp {
		once.Do(func() {
			regex = regexp.MustCompile(str)
		})
		return regex
	}
}

func isCoinTicker(fl validator.FieldLevel) bool {
	return coinTickerRegex().MatchString(fl.Field().String())
}

func isSolanaAddress(fl validator.FieldLevel) bool {
	_, err := solana.PublicKeyFromBase58(fl.Field().String())
	return err == nil
}

func isUnicodeName(fl validator.FieldLevel) bool {
	return unicodeNameRegex().MatchString(fl.Field().String())
}
