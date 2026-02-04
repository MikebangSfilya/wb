package validator

import (
	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func Init() {
	validate = validator.New()
}

func Validate(order *model.Order) error {
	return validate.Struct(order)
}
