package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/vocfaucet/storage"
	"go.vocdoni.io/dvote/api"
	vfaucet "go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain"
)

type faucet struct {
	signer     *ethereum.SignKeys
	authTypes  map[string]uint64
	waitPeriod time.Duration
	storage    *storage.Storage
}

// prepareFaucetPackage prepares a faucet package, including the signature, for the given address.
// Returns the faucet package as a marshaled json byte array, ready to be sent to the user.
func (f *faucet) prepareFaucetPackage(toAddr common.Address, authTypeName string) (*vfaucet.FaucetResponse, error) {
	// check if the auth type is supported
	if _, ok := f.authTypes[authTypeName]; !ok {
		return nil, fmt.Errorf("auth type %s not supported", authTypeName)
	}

	// generate faucet package
	fpackage, err := vochain.GenerateFaucetPackage(f.signer, toAddr, f.authTypes[authTypeName])
	if err != nil {
		return nil, api.ErrCantGenerateFaucetPkg.WithErr(err)
	}
	fpackageBytes, err := json.Marshal(vfaucet.FaucetPackage{
		FaucetPayload: fpackage.Payload,
		Signature:     fpackage.Signature,
	})
	if err != nil {
		return nil, err
	}
	// send response
	return &vfaucet.FaucetResponse{
		Amount:        fmt.Sprint(f.authTypes[authTypeName]),
		FaucetPackage: fpackageBytes,
	}, nil
}
