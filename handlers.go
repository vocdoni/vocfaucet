package main

import (
	"encoding/json"
	"fmt"

	"github.com/vocdoni/vocfaucet/oauthhandler"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
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
			"/oauth/claim/{provider}/{code}/{to}",
			"GET",
			apirest.MethodAccessTypePublic,
			f.authOAuthHandler,
		); err != nil {
			log.Fatal(err)
		}

		if err := api.RegisterMethod(
			"/oauth/authUrl/{provider}",
			"POST",
			apirest.MethodAccessTypePublic,
			f.authOAuthUrl,
		); err != nil {
			log.Fatal(err)
		}
	}
}

// Returns the list of supported auth types
func (f *faucet) authTypesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	data, err := json.Marshal(
		&AuthTypes{
			AuthTypes: f.authTypes,
		},
	)
	if err != nil {
		panic(err) // should not happen
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
	if funded, t := f.storage.checkIsFundedAddress(addr); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(HandlerResponse).SetError(errReason).MustMarshall(), CodeErrFlood)
	}
	data, err := f.prepareFaucetPackage(addr, "open")
	if err != nil {
		return err
	}
	if err := f.storage.addFundedAddress(addr); err != nil {
		return err
	}
	return ctx.Send(new(HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// oAuth faucet handler
func (f *faucet) authOAuthHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.authTypes["oauth"]
	if !ok || amount == 0 {
		return ctx.Send([]byte("auth type oAuth not supported"), apirest.HTTPstatusInternalErr)
	}
	addr, err := stringToAddress(ctx.URLParam("to"))
	if err != nil {
		return err
	}
	if funded, t := f.storage.checkIsFundedAddress(addr); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(HandlerResponse).SetError(errReason).MustMarshall(), CodeErrFlood)
	}

	// Convert the provided "code" to an oAuth Token
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrInitProviders).MustMarshall(), CodeErrInitProviders)
	}

	requestedProvider := ctx.URLParam("provider")
	oAuthCode := ctx.URLParam("code")
	redirectURL := ctx.URLParam("redirectURL")
	provider, ok := providers[requestedProvider]
	if !ok {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrOauthProviderNotFound).MustMarshall(), CodeErrOauthProviderNotFound)
	}

	_, err = provider.GetOAuthToken(oAuthCode, redirectURL)
	if err != nil {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrOauthProviderError).MustMarshall(), CodeErrOauthProviderError)
	}

	data, err := f.prepareFaucetPackage(addr, "oauth")
	if err != nil {
		return err
	}
	if err := f.storage.addFundedAddress(addr); err != nil {
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

	requestedProvider := ctx.URLParam("provider")

	type r struct {
		RedirectURL string `json:"redirectURL"`
	}
	newAuthUrlRequest := r{}
	if err := json.Unmarshal(msg.Data, &newAuthUrlRequest); err != nil {
		return err
	}

	redirectURL := newAuthUrlRequest.RedirectURL
	provider, ok := providers[requestedProvider]
	if !ok {
		return ctx.Send(new(HandlerResponse).SetError(ReasonErrOauthProviderNotFound).MustMarshall(), CodeErrOauthProviderNotFound)
	}

	type urlResponse struct{
		Url string `json:"url"`
	}
	authURL := urlResponse{Url: provider.GetAuthURL(redirectURL)}
	return ctx.Send(new(HandlerResponse).Set(authURL).MustMarshall(), apirest.HTTPstatusOK)
}
