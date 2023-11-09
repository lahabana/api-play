// Package api provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen/v2 version v2.0.0 DO NOT EDIT.
package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oapi-codegen/runtime"
)

// APIResponse defines model for APIResponse.
type APIResponse struct {
	Body          string        `json:"body"`
	Calls         []CallOutcome `json:"calls"`
	LatencyMillis int           `json:"latency_millis" yaml:"latency_millis"`
	Status        int           `json:"status"`
}

// CallDef a list of urls that we'd call get on
type CallDef struct {
	// IgnoreStatus don't consider the status code when using `inherit`
	IgnoreStatus bool `json:"ignore_status" yaml:"ignore_status"`

	// TrimBody don't include the response body in the response of the parent API
	TrimBody bool   `json:"trim_body" yaml:"trim_body"`
	Url      string `json:"url"`
}

// CallOutcome defines model for CallOutcome.
type CallOutcome struct {
	Body   *string `json:"body,omitempty"`
	Status int     `json:"status"`
	Url    string  `json:"url"`
}

// ConfigureAPI defines model for ConfigureAPI.
type ConfigureAPI struct {
	// Body The content to return in the response
	Body string    `json:"body"`
	Call []CallDef `json:"call"`

	// Latency Extra latency to pick from a uniform distribution to add to this call
	Latency *LatencyDef `json:"latency,omitempty"`

	// Statuses The status codes to return, it will return with the probability passed in,
	// If the sum of the ratio of the entries doesn't add to 100000 it will complete with the status
	// of the children calls or 200 if there were no children calls
	Statuses []StatusDef `json:"statuses"`
}

// ConfigureAPIItem defines model for ConfigureAPIItem.
type ConfigureAPIItem struct {
	Conf ConfigureAPI `json:"conf"`
	Path string       `json:"path"`
}

// ErrorResponse defines model for ErrorResponse.
type ErrorResponse struct {
	Details           string               `json:"details"`
	InvalidParameters *[]InvalidParameters `json:"invalid_parameters,omitempty" yaml:"invalid_parameters"`
	Status            float32              `json:"status"`
}

// Health defines model for Health.
type Health struct {
	Status int `json:"status"`
}

// HomeResponse defines model for HomeResponse.
type HomeResponse struct {
	Commit   string `json:"commit"`
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	Version  string `json:"version"`
}

// InvalidParameters defines model for InvalidParameters.
type InvalidParameters struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// LatencyDef Extra latency to pick from a uniform distribution to add to this call
type LatencyDef struct {
	MaxMillis int `json:"max_millis" yaml:"max_millis"`
	MinMillis int `json:"min_millis" yaml:"min_millis"`
}

// ParamsAPI defines model for ParamsAPI.
type ParamsAPI struct {
	Apis []ConfigureAPIItem `json:"apis"`
}

// StatusDef defines model for StatusDef.
type StatusDef struct {
	// Code The status code to return. `inherit` is a special key that will return whatever `call` leads to
	Code string `json:"code"`

	// Ratio The proportion of the requests out of 10k that should return this status
	Ratio int `json:"ratio"`
}

// ConfigureApiJSONRequestBody defines body for ConfigureApi for application/json ContentType.
type ConfigureApiJSONRequestBody = ConfigureAPI

// DegradeHealthJSONRequestBody defines body for DegradeHealth for application/json ContentType.
type DegradeHealthJSONRequestBody = Health

// DegradeReadyJSONRequestBody defines body for DegradeReady for application/json ContentType.
type DegradeReadyJSONRequestBody = Health

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// home
	// (GET /)
	Home(c *gin.Context)
	// list all apis registered
	// (GET /api/dynamic)
	ParamsApi(c *gin.Context)
	// hello
	// (GET /api/dynamic/{path})
	GetApi(c *gin.Context, path string)
	// set api params
	// (POST /api/dynamic/{path})
	ConfigureApi(c *gin.Context, path string)
	// healthcheck
	// (GET /health)
	Health(c *gin.Context)
	// change healthcheck response
	// (POST /health)
	DegradeHealth(c *gin.Context)
	// healthcheck
	// (GET /ready)
	Ready(c *gin.Context)
	// change healthcheck response
	// (POST /ready)
	DegradeReady(c *gin.Context)
}

// ServerInterfaceWrapper converts contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler            ServerInterface
	HandlerMiddlewares []MiddlewareFunc
	ErrorHandler       func(*gin.Context, error, int)
}

type MiddlewareFunc func(c *gin.Context)

// Home operation middleware
func (siw *ServerInterfaceWrapper) Home(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.Home(c)
}

// ParamsApi operation middleware
func (siw *ServerInterfaceWrapper) ParamsApi(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.ParamsApi(c)
}

// GetApi operation middleware
func (siw *ServerInterfaceWrapper) GetApi(c *gin.Context) {

	var err error

	// ------------- Path parameter "path" -------------
	var path string

	err = runtime.BindStyledParameter("simple", false, "path", c.Param("path"), &path)
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter path: %w", err), http.StatusBadRequest)
		return
	}

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.GetApi(c, path)
}

// ConfigureApi operation middleware
func (siw *ServerInterfaceWrapper) ConfigureApi(c *gin.Context) {

	var err error

	// ------------- Path parameter "path" -------------
	var path string

	err = runtime.BindStyledParameter("simple", false, "path", c.Param("path"), &path)
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter path: %w", err), http.StatusBadRequest)
		return
	}

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.ConfigureApi(c, path)
}

// Health operation middleware
func (siw *ServerInterfaceWrapper) Health(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.Health(c)
}

// DegradeHealth operation middleware
func (siw *ServerInterfaceWrapper) DegradeHealth(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.DegradeHealth(c)
}

// Ready operation middleware
func (siw *ServerInterfaceWrapper) Ready(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.Ready(c)
}

// DegradeReady operation middleware
func (siw *ServerInterfaceWrapper) DegradeReady(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.DegradeReady(c)
}

// GinServerOptions provides options for the Gin server.
type GinServerOptions struct {
	BaseURL      string
	Middlewares  []MiddlewareFunc
	ErrorHandler func(*gin.Context, error, int)
}

// RegisterHandlers creates http.Handler with routing matching OpenAPI spec.
func RegisterHandlers(router gin.IRouter, si ServerInterface) {
	RegisterHandlersWithOptions(router, si, GinServerOptions{})
}

// RegisterHandlersWithOptions creates http.Handler with additional options
func RegisterHandlersWithOptions(router gin.IRouter, si ServerInterface, options GinServerOptions) {
	errorHandler := options.ErrorHandler
	if errorHandler == nil {
		errorHandler = func(c *gin.Context, err error, statusCode int) {
			c.JSON(statusCode, gin.H{"msg": err.Error()})
		}
	}

	wrapper := ServerInterfaceWrapper{
		Handler:            si,
		HandlerMiddlewares: options.Middlewares,
		ErrorHandler:       errorHandler,
	}

	router.GET(options.BaseURL+"/", wrapper.Home)
	router.GET(options.BaseURL+"/api/dynamic", wrapper.ParamsApi)
	router.GET(options.BaseURL+"/api/dynamic/:path", wrapper.GetApi)
	router.POST(options.BaseURL+"/api/dynamic/:path", wrapper.ConfigureApi)
	router.GET(options.BaseURL+"/health", wrapper.Health)
	router.POST(options.BaseURL+"/health", wrapper.DegradeHealth)
	router.GET(options.BaseURL+"/ready", wrapper.Ready)
	router.POST(options.BaseURL+"/ready", wrapper.DegradeReady)
}
