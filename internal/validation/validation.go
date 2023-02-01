package validation

import (
	"github.com/go-playground/validator/v10"

	"github.com/sergeii/swat4master/internal/validation/validators"
)

var (
	Validate *validator.Validate
)

func Register() error {
	Validate = validator.New()
	if err := Validate.RegisterValidation("ratio", validators.ValidateRatio); err != nil {
		return err
	}
	return nil
}
