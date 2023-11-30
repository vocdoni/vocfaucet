package main

import "fmt"

// AuthTypes is a struct to return the supported authentication types.
type AuthTypes struct {
	AuthTypes   map[string]uint64 `json:"auth"`
	WaitSeconds uint64            `json:"waitSeconds"`
}

var errAddressAlreadyFunded = fmt.Errorf("address already funded")

type ErrorResponse struct {
	Error string `json:"error"`
}
