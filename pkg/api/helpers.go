package api

import (
	"errors"
	"fmt"
	api_errors "github.com/lahabana/test-api/pkg/errors"
	"net/http"
	"regexp"
	"strconv"
)

var rePath = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_-]+$")

func ValidatePath(path string) error {
	if !rePath.MatchString(path) {
		return fmt.Errorf("'%s' doesn't match re: %s", path, rePath.String())
	}
	return nil
}

func (a *ParamsAPI) Validate() error {
	r := &api_errors.MultiValidationError{}
	for i, api := range a.Apis {
		r = r.AddRootedAt(api.Validate(), "apis", i)
	}
	return r.OrNil()
}

func (a *ParamsAPI) Normalize() {
	for i := range a.Apis {
		a.Apis[i].Normalize()
	}
}

func (a *ConfigureAPIItem) Validate() error {
	r := &api_errors.MultiValidationError{}
	return r.AddRootedAt(ValidatePath(a.Path), "path").
		AddRootedAt(a.Conf.Validate(), "conf").
		OrNil()
}

func (a *ConfigureAPIItem) Normalize() {
	a.Conf.Normalize()
}

func (a CallDef) Validate() error {
	r := &api_errors.MultiValidationError{}
	if a.Url == "" {
		r = r.AddRootedAt("can't be empty", "url")
	}
	return r.OrNil()
}

func (a StatusDef) Validate() error {
	merr := &api_errors.MultiValidationError{}
	if a.Code != "inherit" {
		code, err := strconv.Atoi(a.Code)
		if err != nil {
			merr = merr.AddRootedAt("is not a number or `inherit`", "code")
		} else if code <= 0 {
			merr = merr.AddRootedAt("must be greater than 0", "code")
		}
	}
	if a.Ratio <= 0 || a.Ratio > 100000 {
		merr = merr.AddRootedAt("must be between 1 and 100,000", "ratio")
	}
	return merr.OrNil()
}

func (a *LatencyDef) Validate() error {
	if a == nil {
		return nil
	}
	merr := &api_errors.MultiValidationError{}
	if a.MaxMillis < a.MinMillis {
		merr = merr.AddRootedAt("must have max_millis >= min_millis")
	}
	if a.MinMillis < 0 {
		merr = merr.AddRootedAt("can't be negative", "min_millis")
	}
	if a.MaxMillis < 0 {
		merr = merr.AddRootedAt("can't be negative", "max_millis")
	}
	return merr.OrNil()
}

func (a *ConfigureAPI) Validate() error {
	merr := &api_errors.MultiValidationError{}
	merr = merr.AddRootedAt(a.Latency.Validate(), "latency")
	for i, c := range a.Call {
		merr = merr.AddRootedAt(c.Validate(), "call", i)
	}
	total := 0
	allStatus := map[string]struct{}{}
	for i, s := range a.Statuses {
		merr = merr.AddRootedAt(s.Validate(), "statuses", i)
		total += s.Ratio
		if s.Code == "inherit" && len(a.Call) == 0 {
			merr = merr.AddRootedAt("can't use status with code 'inherit' when no call set", "statuses")
		}
		_, exists := allStatus[s.Code]
		if exists {
			merr = merr.AddRootedAt("duplicate status", "statuses", i)
		}
	}
	if total > 100000 {
		merr = merr.AddRootedAt("sum of ratios can't be greater than 100,000", "statuses")
	}
	return merr.OrNil()
}

func (a *ConfigureAPI) Normalize() {
	if a.Call == nil {
		a.Call = []CallDef{}
	}
	if a.Statuses == nil {
		a.Statuses = []StatusDef{}
	}
}

func BadRequestResponse(err error) ErrorResponse {
	res := ErrorResponse{Status: http.StatusBadRequest}
	t := &api_errors.MultiValidationError{}
	if errors.As(err, &t) {
		res.Details = "Validation errors"
		var valErrors []InvalidParameters
		for _, e := range t.Errors {
			valErrors = append(valErrors, InvalidParameters{Field: e.Path(), Reason: e.Message})
		}
		res.InvalidParameters = &valErrors
	} else {
		res.Details = fmt.Sprintf("Failed with error: %s", err.Error())
	}
	return res
}
