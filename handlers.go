package main

import (
	"encoding/json"
	"fmt"

	"github.com/vocdoni/vocfaucet/oauthhandler"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

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

func (f *faucet) authTypesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	data, err := json.Marshal(
		&AuthTypes{
			AuthTypes: f.authTypes,
		},
	)
	if err != nil {
		panic(err) // should not happen
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

func (f *faucet) authOpenHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.authTypes["open"]
	if !ok || amount == 0 {
		return fmt.Errorf("auth type open not supported")
	}
	addr, err := stringToAddress(ctx.URLParam("to"))
	if err != nil {
		return err
	}
	if funded, t := f.storage.checkIsFundedAddress(addr); funded {
		return fmt.Errorf("address %s already funded, wait until %s", addr.Hex(), t)
	}
	data, err := f.prepareFaucetPackage(addr, "open")
	if err != nil {
		return err
	}
	if err := f.storage.addFundedAddress(addr); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

func (f *faucet) authOAuthHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.authTypes["oauth"]
	if !ok || amount == 0 {
		return fmt.Errorf("auth type oAuth not supported")
	}
	addr, err := stringToAddress(ctx.URLParam("to"))
	if err != nil {
		return err
	}
	if funded, t := f.storage.checkIsFundedAddress(addr); funded {
		return fmt.Errorf("address %s already funded, wait until %s", addr.Hex(), t)
	}

	// Convert the provided "code" to an oAuth Token
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		return fmt.Errorf("error oAuth initializing providers")
	}

	requestedProvider := ctx.URLParam("provider")
	oAuthCode := ctx.URLParam("code")
	redirectURL := ctx.URLParam("redirectURL")
	provider, ok := providers[requestedProvider]
	if !ok {
		return fmt.Errorf("provider not found")
	}

	oAuthToken, err := provider.GetOAuthToken(oAuthCode, redirectURL)
	if err != nil {
		return fmt.Errorf("error obtaining the oAuthToken")
	}
	fmt.Println("Obtained oAuthToken: ", oAuthToken)

	data, err := f.prepareFaucetPackage(addr, "oauth")
	if err != nil {
		return err
	}
	if err := f.storage.addFundedAddress(addr); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

func (f *faucet) authOAuthUrl(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		log.Warnw("error oAuth initializing providers", "err", err)
		return fmt.Errorf("error oAuth initializing providers")
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
		log.Warnw("provider not found", "requestedProvider", requestedProvider)
		return fmt.Errorf("provider not found")
	}

	authURL := provider.GetAuthURL(redirectURL)
	return ctx.Send([]byte(authURL), apirest.HTTPstatusOK)
}
