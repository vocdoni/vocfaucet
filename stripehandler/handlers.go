package stripehandler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	hr "github.com/vocdoni/vocfaucet/handlersresponse"
	"github.com/vocdoni/vocfaucet/helpers"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

// Register the handlers URLs
func (s *StripeHandler) RegisterHandlers(api *apirest.API) {
	if err := api.RegisterMethod(
		"/createCheckoutSession/{to}",
		"POST",
		apirest.MethodAccessTypePublic,
		s.createCheckoutSession,
	); err != nil {
		log.Fatal(err)
	}

	if err := api.RegisterMethod(
		"/createCheckoutSession/{to}/{amount}",
		"POST",
		apirest.MethodAccessTypePublic,
		s.createCheckoutSession,
	); err != nil {
		log.Fatal(err)
	}

	if err := api.RegisterMethod(
		"/sessionStatus/{session_id}",
		"GET",
		apirest.MethodAccessTypePublic,
		s.retrieveCheckoutSession,
	); err != nil {
		log.Fatal(err)
	}

	if err := api.RegisterMethod(
		"/webhook",
		"POST",
		apirest.MethodAccessTypePublic,
		s.handleWebhook,
	); err != nil {
		log.Fatal(err)
	}
}

// createCheckoutSession creates a new Stripe Checkout session
func (s *StripeHandler) createCheckoutSession(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	to := ctx.URLParam("to")
	// referral := ctx.URLParam("referral")
	defaultAmount := s.DefaultAmount
	if amount := ctx.URLParam("amount"); amount != "" {
		var err error
		defaultAmount, err = strconv.ParseInt(amount, 10, 64)
		if err != nil {
			return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrIncorrectParams)
		}
	}
	type r struct {
		ReturnURL string `json:"returnURL"`
		Referral  string `json:"referral"`
	}
	newRequest := r{}
	if err := json.Unmarshal(msg.Data, &newRequest); err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrIncorrectParams)
	}
	sess, err := s.CreateCheckoutSession(defaultAmount, to, newRequest.ReturnURL, newRequest.Referral)
	if err != nil {
		errReason := fmt.Sprintf("session.New: %v", err)
		return ctx.Send(new(hr.HandlerResponse).SetError(errReason).MustMarshall(), hr.CodeErrProviderError)
	}
	data := &struct {
		ClientSecret string `json:"clientSecret"`
	}{
		ClientSecret: sess.ClientSecret,
	}
	return ctx.Send(new(hr.HandlerResponse).Set(data).MustMarshall(), apirest.HTTPstatusOK)
}

func (s *StripeHandler) retrieveCheckoutSession(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	sessionId := ctx.URLParam("session_id")
	status, err := s.RetrieveCheckoutSession(sessionId)
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrProviderError)
	}
	_, err = s.Storage.Get([]byte(sessionId))
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrInternalError)
	}
	data, err := s.processPaymentTransfer(status.Quantity, status.Recipient)
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrInternalError)
	}
	if err := s.Storage.Delete([]byte(sessionId)); err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), hr.CodeErrInternalError)
	}
	status.FaucetPackage = data
	return ctx.Send(new(hr.HandlerResponse).Set(status).MustMarshall(), apirest.HTTPstatusOK)
}

func (s *StripeHandler) handleWebhook(apiData *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	sig := ctx.Request.Header.Get("Stripe-Signature")
	// Pass the request body and Stripe-Signature header to ConstructEvent, along with the webhook signing key
	sessionId, err := s.HandleWebhook(apiData, sig)
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), http.StatusBadRequest)
	}
	err = s.Storage.Set([]byte(sessionId), nil)
	if err != nil {
		return ctx.Send(new(hr.HandlerResponse).SetError(err.Error()).MustMarshall(), http.StatusBadRequest)
	}
	return ctx.Send([]byte("success"), http.StatusOK)
}

func (s *StripeHandler) processPaymentTransfer(amount int64, to string) ([]byte, error) {
	if amount == 0 {
		return nil, fmt.Errorf("invalid requested amount")
	}
	addr, err := helpers.StringToAddress(to)
	if err != nil {
		return nil, err
	}
	data, err := s.Faucet.PrepareFaucetPackageWithAmount(addr, uint64(amount))
	if err != nil {
		return nil, err
	}

	return data.FaucetPackage, nil
}
