package core

import (
	"reflect"

	"github.com/gin-gonic/gin/binding"
	"github.com/Neur0toxine/mg-transport-lib/internal"
	"gopkg.in/go-playground/validator.v8"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("validatecrmurl", validateCrmURL); err != nil {
			panic("cannot register crm url validator: " + err.Error())
		}
	}
}

func validateCrmURL(
	v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value,
	field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string,
) bool {
	return internal.RegCommandName.Match([]byte(field.Interface().(string)))
}
