package faucet

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/vocfaucet/storage"
	"go.vocdoni.io/dvote/api"
	vFaucet "go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain"
)

type Faucet struct {
	Signer     *ethereum.SignKeys
	AuthTypes  map[string]uint64
	WaitPeriod time.Duration
	Storage    *storage.Storage
}

// prepareFaucetPackage prepares a Faucet package, including the signature, for the given address.
// Returns the Faucet package as a marshaled json byte array, ready to be sent to the user.
func (f *Faucet) prepareFaucetPackage(toAddr common.Address, authTypeName string) (*vFaucet.FaucetResponse, error) {
	// check if the auth type is supported
	if _, ok := f.AuthTypes[authTypeName]; !ok {
		return nil, fmt.Errorf("auth type %s not supported", authTypeName)
	}

	// generate Faucet package
	fpackage, err := vochain.GenerateFaucetPackage(f.Signer, toAddr, f.AuthTypes[authTypeName])
	if err != nil {
		return nil, api.ErrCantGenerateFaucetPkg.WithErr(err)
	}
	fpackageBytes, err := json.Marshal(vFaucet.FaucetPackage{
		FaucetPayload: fpackage.Payload,
		Signature:     fpackage.Signature,
	})
	if err != nil {
		return nil, err
	}
	// send response
	return &vFaucet.FaucetResponse{
		Amount:        fmt.Sprint(f.AuthTypes[authTypeName]),
		FaucetPackage: fpackageBytes,
	}, nil
}

// PrepareFaucetPackageWithAmount prepares a Faucet package, including the signature, for the given address.
// Returns the Faucet package as a marshaled json byte array, ready to be sent to the user.
func (f *Faucet) PrepareFaucetPackageWithAmount(toAddr common.Address, amount uint64) (*vFaucet.FaucetResponse, error) {
	if amount == 0 {
		return nil, fmt.Errorf("invalid requested amount: %d", amount)
	}

	// generate Faucet package
	fpackage, err := vochain.GenerateFaucetPackage(f.Signer, toAddr, amount)
	if err != nil {
		return nil, api.ErrCantGenerateFaucetPkg.WithErr(err)
	}
	fpackageBytes, err := json.Marshal(vFaucet.FaucetPackage{
		FaucetPayload: fpackage.Payload,
		Signature:     fpackage.Signature,
	})
	if err != nil {
		return nil, err
	}
	// send response
	return &vFaucet.FaucetResponse{
		Amount:        fmt.Sprint(amount),
		FaucetPackage: fpackageBytes,
	}, nil
}
