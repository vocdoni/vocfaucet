package faucet

import "fmt"

// AuthTypes is a struct to return the supported authentication types.
type AuthTypes struct {
	AuthTypes   map[string]uint64 `json:"auth"`
	WaitSeconds uint64            `json:"waitSeconds"`
}

var errAddressAlreadyFunded = fmt.Errorf("address already funded")

const (
	AuthTypeOpen      = "open"
	AuthTypeOauth     = "oauth"
	AuthTypeAragonDao = "aragondao"
)

type ErrorResponse struct {
	Error string `json:"error"`
}
