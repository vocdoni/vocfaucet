package main

import (
	"encoding/json"
)

const (
	CodeErrUnsupportedAuthType     = 401
	ReasonErrUnsupportedAuthType   = "unsupported auth type"
	CodeErrFlood                   = 402
	CodeErrInitProviders           = 403
	ReasonErrInitProviders         = "error oAuth initializing providers"
	CodeErrOauthProviderNotFound   = 404
	ReasonErrOauthProviderNotFound = "oAuth provider not found"
	CodeErrOauthProviderError      = 405
	ReasonErrOauthProviderError    = "error obtaining the oAuthToken"
	CodeErrAragonDaoSignature      = 406
	CodeErrAragonDaoAddress        = 407
	CodeErrIncorrectParams         = 408
	CodeErrInternalError           = 409
	ReasonErrAragonDaoAddress      = "could not find the signer address in any Aragon DAO"
)

// HandlerResponse is the response format for the Handlers
type HandlerResponse struct {
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// Set sets the data for a successful response
func (e *HandlerResponse) Set(data any) *HandlerResponse {
	e.Data = data
	return e
}

// SetError sets the error code and reason for a failed response
func (e *HandlerResponse) SetError(reason string) *HandlerResponse {
	e.Error = reason
	return e
}

// MustMarshall marshalls the response and panics if it fails
func (e *HandlerResponse) MustMarshall() []byte {
	var data []byte
	var err error
	if e.Error != "" {
		data, err = json.Marshal(e)
		if err != nil {
			panic(err)
		}
	} else {
		data, err = json.Marshal(e.Data)
		if err != nil {
			panic(err)
		}
	}
	return data
}
