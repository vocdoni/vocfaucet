package main

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

func (f *faucet) registerHandlers(api *apirest.API) {
	if err := api.RegisterMethod(
		"/authTypes",
		"GET",
		apirest.MethodAccessTypePublic,
		f.authTypesHAndler,
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

}

func (f *faucet) authTypesHAndler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
