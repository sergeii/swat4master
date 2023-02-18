package validation

import (
	"github.com/go-playground/validator/v10"

	"github.com/sergeii/swat4master/internal/validation/validators"
)

func New() (*validator.Validate, error) {
	validate := validator.New()
	if err := validate.RegisterValidation("ratio", validators.ValidateRatio); err != nil {
		return nil, err
	}
	return validate, nil
}
