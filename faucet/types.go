package faucet

// AuthTypes is a struct to return the supported authentication types.
type AuthTypes struct {
	AuthTypes   map[string]uint64 `json:"auth"`
	WaitSeconds uint64            `json:"waitSeconds"`
}

const (
	AuthTypeOpen      = "open"
	AuthTypeOauth     = "oauth"
	AuthTypeAragonDao = "aragondao"
	AuthTypeStripe    = "stripe"
)

type ErrorResponse struct {
	Error string `json:"error"`
}
