package errors

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type Validate interface {
	Validate() error
}

type ValidationError struct {
	Fields  []interface{}
	Message string
}

type MultiValidationError struct {
	Errors []ValidationError
}

func (m *MultiValidationError) OrNil() error {
	if len(m.Errors) == 0 {
		return nil
	}
	return m
}

func (m *MultiValidationError) AddRootedAt(in any, path ...interface{}) *MultiValidationError {
	if in == nil {
		return m
	}
	s, ok := in.(string)
	if ok {
		m.Errors = append(m.Errors, ValidationError{Fields: path, Message: s})
		return m
	}
	err, ok := in.(error)
	if !ok {
		panic(fmt.Sprintf("Can't do this as entry is not an error type %+v", reflect.TypeOf(in)))
	}

	verr := &ValidationError{}
	if errors.As(err, &verr) {
		m.Errors = append(m.Errors, ValidationError{
			Fields:  append(path, verr.Fields...),
			Message: verr.Message,
		})
		return m
	}

	oerr := &MultiValidationError{}
	if errors.As(err, &oerr) {
		for _, childErr := range oerr.Errors {
			m.Errors = append(m.Errors, ValidationError{
				Fields:  append(path, childErr.Fields...),
				Message: childErr.Message,
			})
		}
		return m
	}
	m.Errors = append(m.Errors, ValidationError{
		Fields:  path,
		Message: err.Error(),
	})
	return m
}

func (m *MultiValidationError) Error() string {
	var allErrors []string
	for _, e := range m.Errors {
		allErrors = append(allErrors, e.Error())
	}
	return "failed validation with Errors: " + strings.Join(allErrors, ",")
}

func (m *MultiValidationError) As(target interface{}) bool {
	o, ok := target.(*MultiValidationError)
	if !ok {
		return false
	}
	o.Errors = append(o.Errors, m.Errors...)
	return true
}

func (v *ValidationError) Path() string {
	sb := strings.Builder{}
	for _, f := range v.Fields {
		switch f.(type) {
		case int:
			sb.WriteString(fmt.Sprintf("[%d]", f))
		default:
			sb.WriteString(fmt.Sprintf(".%s", f))
		}
	}
	return sb.String()
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("field: %s, failed validation with error: %s", v.Path(), v.Message)
}

func (v *ValidationError) As(target interface{}) bool {
	o, ok := target.(*ValidationError)
	if !ok {
		return false
	}
	o.Message = v.Message
	o.Fields = append(o.Fields, v.Fields...)
	return true
}
