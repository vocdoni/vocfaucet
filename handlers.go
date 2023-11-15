package main

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/vocfaucet/aragondaohandler"
	"github.com/vocdoni/vocfaucet/oauthhandler"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// Register the handlers URLs
func (f *faucet) registerHandlers(api *apirest.API) {
	if err := api.RegisterMethod(
		"/authTypes",
		"GET",
		apirest.MethodAccessTypePublic,
		f.authTypesHandler,
	); err != nil {
		log.Fatal(err)
	}

	if f.authTypes["open"] > 0 {
		if err := api.RegisterMethod(
			"/open/claim/{to}",
			"GET",
			apirest.MethodAccessTypePublic,
			f.authOpenHandler,
		); err != nil {
			log.Fatal(err)
		}
	}

	if f.authTypes["oauth"] > 0 {
		if err := api.RegisterMethod(
			"/oauth/claim",
			"POST",
			apirest.MethodAccessTypePublic,
			f.authOAuthHandler,
		); err != nil {
			log.Fatal(err)
		}

		if err := api.RegisterMethod(
			"/oauth/authUrl",
			"POST",
			apirest.MethodAccessTypePublic,
			f.authOAuthUrl,
		); err != nil {
			log.Fatal(err)
		}
	}

	if f.authTypes["aragondao"] > 0 {
		if err := api.RegisterMethod(
			"/aragondao/claim",
			"POST",
			apirest.MethodAccessTypePublic,
			f.authAragonDaoHandler,
		); err != nil {
			log.Fatal(err)
		}
	}
}

// Returns the list of supported auth types
func (f *faucet) authTypesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	data := &AuthTypes{
		AuthTypes: f.authTypes,
	}
	return ctx.Send(new(HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// Open faucet handler (does no logic but flood protection)
func (f *faucet) authOpenHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.authTypes["open"]
	if !ok || amount == 0 {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrUnsupportedAuthType).MustMarshall(), CodeErrUnsupportedAuthType)
	}
	addr, err := stringToAddress(ctx.URLParam("to"))
	if err != nil {
		return err
	}
	if funded, t := f.storage.checkIsFundedAddress(addr, "open"); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(HandlerResponse).SetError(errReason).MustMarshall(), CodeErrFlood)
	}
	data, err := f.prepareFaucetPackage(addr, "open")
	if err != nil {
		return err
	}
	if err := f.storage.addFundedAddress(addr, "open"); err != nil {
		return err
	}
	return ctx.Send(new(HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// oAuth faucet handler
func (f *faucet) authOAuthHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.authTypes["oauth"]
	if !ok || amount == 0 {
		return ctx.Send([]byte("auth type oAuth not supported"), apirest.HTTPstatusInternalErr)
	}

	type r struct {
		Provider    string `json:"provider"`
		Code        string `json:"code"`
		RedirectURL string `json:"redirectURL"`
		Recipient   string `json:"recipient"`
	}
	newRequest := r{}
	if err := json.Unmarshal(msg.Data, &newRequest); err != nil {
		return ctx.Send(new(HandlerResponse).SetError(err.Error()).MustMarshall(), CodeErrIncorrectParams)
	}

	addr, err := stringToAddress(newRequest.Recipient)
	if err != nil {
		return err
	}
	if funded, t := f.storage.checkIsFundedAddress(addr, "oauth"); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(HandlerResponse).SetError(errReason).MustMarshall(), CodeErrFlood)
	}

	// Convert the provided "code" to an oAuth Token
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrInitProviders).MustMarshall(), CodeErrInitProviders)
	}

	provider, ok := providers[newRequest.Provider]
	if !ok {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrOauthProviderNotFound).MustMarshall(), CodeErrOauthProviderNotFound)
	}

	_, err = provider.GetOAuthToken(newRequest.Code, newRequest.RedirectURL)
	if err != nil {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrOauthProviderError).MustMarshall(), CodeErrOauthProviderError)
	}

	data, err := f.prepareFaucetPackage(addr, "oauth")
	if err != nil {
		return err
	}
	if err := f.storage.addFundedAddress(addr, "oauth"); err != nil {
		return err
	}

	return ctx.Send(new(HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// oAuth faucet handler (returns the oAuth URL)
func (f *faucet) authOAuthUrl(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrInitProviders).MustMarshall(), CodeErrInitProviders)
	}

	type r struct {
		Provider    string `json:"provider"`
		RedirectURL string `json:"redirectURL"`
		State       string `json:"state"`
	}
	newAuthUrlRequest := r{}
	if err := json.Unmarshal(msg.Data, &newAuthUrlRequest); err != nil {
		return ctx.Send(new(HandlerResponse).SetError(err.Error()).MustMarshall(), CodeErrIncorrectParams)
	}

	provider, ok := providers[newAuthUrlRequest.Provider]
	if !ok {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrOauthProviderNotFound).MustMarshall(), CodeErrOauthProviderNotFound)
	}

	type urlResponse struct {
		Url string `json:"url"`
	}
	authURL := urlResponse{Url: provider.GetAuthURL(newAuthUrlRequest.RedirectURL, newAuthUrlRequest.State)}
	return ctx.Send(new(HandlerResponse).Set(authURL).MustMarshall(), apirest.HTTPstatusOK)
}

func (f *faucet) authAragonDaoHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error

	amount, ok := f.authTypes["aragondao"]
	if !ok || amount == 0 {
		return ctx.Send([]byte("auth type AragonDao not supported"), apirest.HTTPstatusInternalErr)
	}

	type r struct {
		Data      string         `json:"data"`
		Signature types.HexBytes `json:"signature"`
		Network   string         `json:"network"`
	}
	newRequest := r{}
	if err := json.Unmarshal(msg.Data, &newRequest); err != nil {
		return ctx.Send(new(HandlerResponse).SetError(err.Error()).MustMarshall(), CodeErrIncorrectParams)
	}

	// Obtains the URL and verifies the signature is from today
	var addr common.Address
	if addr, err = aragondaohandler.VerifyAragonDaoRequest(newRequest.Data, newRequest.Signature); err != nil {
		return ctx.Send(new(HandlerResponse).SetError(err.Error()).MustMarshall(), CodeErrAragonDaoSignature)
	}

	// Check if the address is already funded
	if funded, t := f.storage.checkIsFundedAddress(addr, "aragon"); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(HandlerResponse).SetError(errReason).MustMarshall(), CodeErrFlood)
	}

	// Check if the address is an Aragon DAO address by checking to AragonGraphQL
	if newRequest.Network != "" {
		if isAragonDao, _ := aragondaohandler.IsAragonDaoAddress(addr, newRequest.Network); !isAragonDao {
			return ctx.Send(new(HandlerResponse).SetError(ReasonErrAragonDaoAddress).MustMarshall(), CodeErrAragonDaoAddress)
		}
	} else { // Check all networks
		found := false
		for network := range aragondaohandler.ValidNetworks {
			if isAragonDao, _ := aragondaohandler.IsAragonDaoAddress(addr, network); isAragonDao {
				found = true
				break
			}
		}
		if !found {
			return ctx.Send(new(HandlerResponse).SetError(ReasonErrAragonDaoAddress).MustMarshall(), CodeErrAragonDaoAddress)
		}
	}

	data, err := f.prepareFaucetPackage(addr, "aragondao")
	if err != nil {
		return ctx.Send(new(HandlerResponse).SetError(err.Error()).MustMarshall(), CodeErrInternalError)
	}

	if err := f.storage.addFundedAddress(addr, "aragon"); err != nil {
		return ctx.Send(new(HandlerResponse).SetError(err.Error()).MustMarshall(), CodeErrInternalError)
	}

	return ctx.Send(new(HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}
