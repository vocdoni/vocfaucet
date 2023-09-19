package main

import (
	"encoding/json"
)

const (
	CodeOk = 400

	CodeErrUnsupportedAuthType     = 401
	ReasonErrUnsupportedAuthType   = "unsupported auth type"
	CodeErrFlood                   = 402
	CodeErrInitProviders           = 403
	ReasonErrInitProviders         = "error oAuth initializing providers"
	CodeErrOauthProviderNotFound   = 404
	ReasonErrOauthProviderNotFound = "oAuth provider not found"
	CodeErrOauthProviderError      = 405
)

// HandlerResponse is the response format for the Handlers
type HandlerResponse struct {
	Ok     bool   `json:"ok"`
	Code   int    `json:"code"`
	Reason string `json:"reason"`
	Data   any    `json:"data"`
}

// Set sets the data for a successful response
func (e *HandlerResponse) Set(data any) *HandlerResponse {
	e.Data = data
	e.Code = CodeOk
	e.Ok = true
	return e
}

// SetError sets the error code and reason for a failed response
func (e *HandlerResponse) SetError(code int, reason string) *HandlerResponse {
	e.Code = code
	e.Reason = reason
	e.Ok = false
	return e
}

// MustMarshall marshalls the response and panics if it fails
func (e *HandlerResponse) MustMarshall() []byte {
	data, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}

	return data
}
