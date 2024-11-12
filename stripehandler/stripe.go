package stripehandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/price"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/vocdoni/vocfaucet/faucet"
	"github.com/vocdoni/vocfaucet/storage"
	"go.vocdoni.io/dvote/httprouter/apirest"
)

// StripeHandler represents the configuration for the stripe a provider for handling Stripe payments.
type StripeHandler struct {
	Key           string           // The API key for the Stripe account.
	PriceId       string           // The ID of the price associated with the product.
	MinQuantity   int64            // The minimum quantity allowed for the product.
	MaxQuantity   int64            // The maximum quantity allowed for the product.
	DefaultAmount int64            // The default amount for the product.
	WebhookSecret string           // The secret used to verify Stripe webhook events.
	Storage       *storage.Storage // The storage instance for the faucet.
	Faucet        *faucet.Faucet   // The faucet instance.
	SessionLock   sync.RWMutex     // The lock for the session.
}

// ReturnStatus represents the response status and data returned by the client.
type ReturnStatus struct {
	Status        string `json:"status"`
	CustomerEmail string `json:"customer_email"`
	FaucetPackage []byte `json:"faucet_package"`
	Recipient     string `json:"recipient"`
	Quantity      int64  `json:"quantity"`
}

// NewStripeClient creates a new instance of the StripeHandler struct with the provided parameters.
// It sets the Stripe API key, price ID, webhook secret, minimum quantity, maximum quantity, and default amount.
// Returns a pointer to the created StripeHandler.
func NewStripeClient(key, priceId, webhookSecret string, minQuantity, maxQuantity, defaultAmount int64, faucet *faucet.Faucet, storage *storage.Storage) (*StripeHandler, error) {
	if key == "" || priceId == "" || webhookSecret == "" || storage == nil {
		return nil, errors.New("missing required parameters")
	}
	stripe.Key = key
	return &StripeHandler{
		PriceId:       priceId,
		MinQuantity:   minQuantity,
		MaxQuantity:   maxQuantity,
		DefaultAmount: defaultAmount,
		WebhookSecret: webhookSecret,
		Storage:       storage,
		Faucet:        faucet,
	}, nil
}

// CreateCheckoutSession creates a new Stripe checkout session.
// It takes the defaultAmount, to, and referral as parameters and returns a pointer to a stripe.CheckoutSession and an error.
// The defaultAmount parameter specifies the default quantity for the checkout session.
// The to parameter is the client reference ID for the checkout session.
// The referral parameter is the referral URL for the checkout session.
// The function constructs a stripe.CheckoutSessionParams object with the provided parameters and creates a new session using the session.New function.
// If the session creation is successful, it returns the session pointer, otherwise it returns an error.
func (s *StripeHandler) CreateCheckoutSession(defaultAmount int64, to, returnURL, referral string) (*stripe.CheckoutSession, error) {
	// search corresponding price tokens package
	packName := fmt.Sprintf("pack_%d", defaultAmount)
	priceParams := &stripe.PriceListParams{Active: stripe.Bool(true), LookupKeys: []*string{stripe.String(packName)}}
	priceList := price.List(priceParams).PriceList()
	// iterate price result
	if len(priceList.Data) == 0 {
		return nil, nil
	}
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(to),
		UIMode:            stripe.String("embedded"),
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		ReturnURL:         stripe.String(returnURL + "/{CHECKOUT_SESSION_ID}"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Product:           &priceList.Data[0].Product.ID,
					Currency:          (*string)(&priceList.Data[0].Currency),
					UnitAmountDecimal: &priceList.Data[0].UnitAmountDecimal,
				},
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"to":       to,
			"referral": referral,
		},
	}
	ses, err := session.New(params)
	if err != nil {
		return nil, err
	}
	return ses, nil
}

// RetrieveCheckoutSession retrieves a checkout session from Stripe by session ID.
// It returns a ReturnStatus object and an error if any.
// The ReturnStatus object contains information about the session status, customer email,
// faucet package, recipient, and quantity.
func (s *StripeHandler) RetrieveCheckoutSession(sessionID string) (*ReturnStatus, error) {
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("line_items")
	sess, err := session.Get(sessionID, params)
	if err != nil {
		return nil, err
	}
	lineItems := sess.LineItems
	data := &ReturnStatus{
		Status:        string(sess.Status),
		CustomerEmail: sess.CustomerDetails.Email,
		FaucetPackage: nil,
		Recipient:     sess.Metadata["to"],
		Quantity:      lineItems.Data[0].Quantity,
	}
	return data, nil
}

// HandleWebhook handles the incoming webhook event from Stripe.
// It takes the API data and signature as input parameters and returns the session ID and an error (if any).
// The request body and Stripe-Signature header are passed to ConstructEvent, along with the webhook signing key.
// If the event type is "checkout.session.completed", it unmarshals the event data into a CheckoutSession struct
// and returns the session ID. Otherwise, it returns an empty string.
func (s *StripeHandler) HandleWebhook(apiData *apirest.APIdata, sig string) (string, error) {
	// Pass the request body and Stripe-Signature header to ConstructEvent, along with the webhook signing key
	event, err := webhook.ConstructEvent(apiData.Data, sig, s.WebhookSecret)
	if err != nil {
		return "", err
	}
	// Handle the checkout.session.completed event
	if event.Type == "checkout.session.completed" {
		var sess stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &sess)
		if err != nil {
			return "", err
		}
		return sess.ID, nil
	}
	return "", nil
}
