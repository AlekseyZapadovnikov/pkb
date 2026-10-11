package valid

import (
	"github.com/go-playground/validator/v10"
)

var Vl *validator.Validate

func init() {
	Vl = validator.New()
}
