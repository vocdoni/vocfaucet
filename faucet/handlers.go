package faucet

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/vocfaucet/aragondaohandler"
	hr "github.com/vocdoni/vocfaucet/handlersresponse"
	"github.com/vocdoni/vocfaucet/helpers"
	"github.com/vocdoni/vocfaucet/oauthhandler"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// Register the handlers URLs
func (f *Faucet) RegisterHandlers(api *apirest.API) {
	if err := api.RegisterMethod(
		"/authTypes",
		"GET",
		apirest.MethodAccessTypePublic,
		f.authTypesHandler,
	); err != nil {
		log.Fatal(err)
	}

	if f.AuthTypes[AuthTypeOpen] > 0 {
		if err := api.RegisterMethod(
			"/open/claim/{to}",
			"GET",
			apirest.MethodAccessTypePublic,
			f.authOpenHandler,
		); err != nil {
			log.Fatal(err)
		}
	}

	if f.AuthTypes[AuthTypeOauth] > 0 {
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

	if f.AuthTypes[AuthTypeAragonDao] > 0 {
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
func (f *Faucet) authTypesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	data := &AuthTypes{
		AuthTypes:   f.AuthTypes,
		WaitSeconds: uint64(f.WaitPeriod.Seconds()),
	}

	return ctx.Send(new(hr.HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// Open Faucet handler (does no logic but flood protection)
func (f *Faucet) authOpenHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.AuthTypes[AuthTypeOpen]
	if !ok || amount == 0 {
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrUnsupportedAuthType).MustMarshall(), hr.CodeErrUnsupportedAuthType)
	}
	addr, err := helpers.StringToAddress(ctx.URLParam("to"))
	if err != nil {
		return err
	}
	if funded, t := f.Storage.CheckFundedUserWithWaitTime(addr.Bytes(), AuthTypeOpen); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(hr.HandlerResponse).SetError(errReason).MustMarshall(), hr.CodeErrFlood)
	}
	data, err := f.prepareFaucetPackage(addr, AuthTypeOpen)
	if err != nil {
		return err
	}
	if err := f.Storage.AddFundedUserWithWaitTime(addr.Bytes(), AuthTypeOpen); err != nil {
		return err
	}
	return ctx.Send(new(hr.HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// oAuth Faucet handler
func (f *Faucet) authOAuthHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	amount, ok := f.AuthTypes[AuthTypeOauth]
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
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrIncorrectParams)
	}

	addr, err := helpers.StringToAddress(newRequest.Recipient)
	if err != nil {
		return err
	}
	if funded, t := f.Storage.CheckFundedUserWithWaitTime(addr.Bytes(), AuthTypeOauth); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(hr.HandlerResponse).SetError(errReason).MustMarshall(), hr.CodeErrFlood)
	}

	// Convert the provided "code" to an oAuth Token
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrInitProviders).MustMarshall(), hr.CodeErrInitProviders)
	}

	provider, ok := providers[newRequest.Provider]
	if !ok {
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrOauthProviderNotFound).MustMarshall(), hr.CodeErrOauthProviderNotFound)
	}

	token, err := provider.GetOAuthToken(newRequest.Code, newRequest.RedirectURL)
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrOauthProviderError).MustMarshall(), hr.CodeErrOauthProviderError)
	}

	profileRaw, err := provider.GetOAuthProfile(token)
	if err != nil {
		log.Warnw("error obtaining the profile", "err", err)
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrOauthProviderError).MustMarshall(), hr.CodeErrOauthProviderError)
	}

	var profile map[string]interface{}
	if err := json.Unmarshal(profileRaw, &profile); err != nil {
		log.Warnw("error marshalling the profile", "err", err)
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrOauthProviderError).MustMarshall(), hr.CodeErrOauthProviderError)
	}

	// Check if the oauth profile is already funded
	fundedProfileField := profile[provider.UsernameField].(string)
	fundedAuthType := "oauth_" + newRequest.Provider
	if funded, t := f.Storage.CheckFundedUserWithWaitTime([]byte(fundedProfileField), fundedAuthType); funded {
		errReason := fmt.Sprintf("user %s already funded, wait until %s", fundedProfileField, t)
		return ctx.Send(new(hr.HandlerResponse).SetError(errReason).MustMarshall(), hr.CodeErrFlood)
	}

	data, err := f.prepareFaucetPackage(addr, AuthTypeOauth)
	if err != nil {
		return err
	}

	// Add address and profile to the funded list
	if err := f.Storage.AddFundedUserWithWaitTime(addr.Bytes(), AuthTypeOauth); err != nil {
		return err
	}
	if err := f.Storage.AddFundedUserWithWaitTime([]byte(fundedProfileField), fundedAuthType); err != nil {
		return err
	}

	return ctx.Send(new(hr.HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

// oAuth Faucet handler (returns the oAuth URL)
func (f *Faucet) authOAuthUrl(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	providers, err := oauthhandler.InitProviders()
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrInitProviders).MustMarshall(), hr.CodeErrInitProviders)
	}

	type r struct {
		Provider    string `json:"provider"`
		RedirectURL string `json:"redirectURL"`
		State       string `json:"state"`
	}
	newAuthUrlRequest := r{}
	if err := json.Unmarshal(msg.Data, &newAuthUrlRequest); err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrIncorrectParams)
	}

	provider, ok := providers[newAuthUrlRequest.Provider]
	if !ok {
		return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrOauthProviderNotFound).MustMarshall(), hr.CodeErrOauthProviderNotFound)
	}

	type urlResponse struct {
		Url string `json:"url"`
	}
	authURL := urlResponse{Url: provider.GetAuthURL(newAuthUrlRequest.RedirectURL, newAuthUrlRequest.State)}
	return ctx.Send(new(hr.HandlerResponse).Set(authURL).MustMarshall(), apirest.HTTPstatusOK)
}

func (f *Faucet) authAragonDaoHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error

	amount, ok := f.AuthTypes[AuthTypeAragonDao]
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
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrIncorrectParams)
	}

	// Obtains the URL and verifies the signature is from today
	var addr common.Address
	if addr, err = aragondaohandler.VerifyAragonDaoRequest(newRequest.Data, newRequest.Signature); err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrAragonDaoSignature)
	}

	// Check if the address is already funded
	if funded, t := f.Storage.CheckFundedUserWithWaitTime(addr.Bytes(), AuthTypeAragonDao); funded {
		errReason := fmt.Sprintf("address %s already funded, wait until %s", addr.Hex(), t)
		return ctx.Send(new(hr.HandlerResponse).SetError(errReason).MustMarshall(), hr.CodeErrFlood)
	}

	// Check if the address is an Aragon DAO address by checking to AragonGraphQL
	if newRequest.Network != "" {
		if isAragonDao, _ := aragondaohandler.IsAragonDaoAddress(addr, newRequest.Network); !isAragonDao {
			return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrAragonDaoAddress).MustMarshall(), hr.CodeErrAragonDaoAddress)
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
			return ctx.Send(new(hr.HandlerResponse).SetError(hr.ReasonErrAragonDaoAddress).MustMarshall(), hr.CodeErrAragonDaoAddress)
		}
	}

	data, err := f.prepareFaucetPackage(addr, AuthTypeAragonDao)
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrInternalError)
	}

	if err := f.Storage.AddFundedUserWithWaitTime(addr.Bytes(), AuthTypeAragonDao); err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrInternalError)
	}

	return ctx.Send(new(hr.HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}
