package main

// AuthTypes is a struct to return the supported authentication types.
type AuthTypes struct {
	AuthTypes map[string]uint64 `json:"auth"`
}
