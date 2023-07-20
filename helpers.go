package main

import (
	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
)

func stringToAddress(addr string) (common.Address, error) {
	if !common.IsHexAddress(addr) {
		return common.Address{}, api.ErrParamToInvalid
	}
	return common.HexToAddress(addr), nil
}
