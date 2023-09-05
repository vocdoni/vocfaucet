package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

var supportedAuthTypes = map[string]string{
	"open":    "without authentication, anyone can use the faucet",
	"oauth":   "with oauth2 authentication",
	"mail":    "with email confirmation (requires twilio)",
	"sms":     "with sms confirmation (requires twilio)",
	"captcha": "with captcha confirmation (requires recaptcha)",
}

func main() {
	logLevel := flag.String("logLevel", "info", "log level")
	tlsDomain := flag.String("tlsDomain", "", "domain for tls (implies listen to port 443)")
	listenHost := flag.String("listenHost", "0.0.0.0", "host to listen on")
	listenPort := flag.Int("listenPort", 8080, "port to listen on")
	baseRoute := flag.String("baseRoute", "/v2", "base route for the API")
	dataDir := flag.String("dataDir", "./vocfaucet-data", "data directory")
	privKey := flag.String("privKey", "", "private key for the faucet signer (hexadecimal)")
	auth := flag.String("auth", "open,oauth", "authentication types to use (comma separated)")
	amounts := flag.String("amounts", "100,200", "tokens to send per request (comma separated), the order must match the auth types")
	waitPeriod := flag.Duration("waitPeriod", 1*time.Hour, "wait period between requests for the same user")
	dbType := flag.StringP("dbType", "t", db.TypePebble, fmt.Sprintf("key-value db type [%s,%s,%s]", db.TypePebble, db.TypeLevelDB, db.TypeMongo))

	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Println("\nAuthentication types supported:")
		for k, v := range supportedAuthTypes {
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Println()
	}

	flag.Parse()
	log.Init(*logLevel, "stdout", nil)

	// parse auth types and amounts
	authNames := strings.Split(*auth, ",")
	for _, t := range authNames {
		if _, ok := supportedAuthTypes[t]; !ok {
			log.Fatalf("unsupported authentication type %s", t)
		}
	}
	amountsStr := strings.Split(*amounts, ",")
	if len(amountsStr) != len(authNames) {
		log.Fatalf("amounts and auth types must have the same length")
	}
	authTypes := make(map[string]uint64, len(authNames))
	for i, a := range amountsStr {
		var err error
		amountUint, err := strconv.ParseUint(a, 10, 64)
		if err != nil {
			log.Fatalf("invalid amount %s", a)
		}
		authTypes[authNames[i]] = amountUint
	}
	log.Infow("enabled authentications and amounts", "types", authTypes)

	// initialize signer
	signer := ethereum.SignKeys{}
	if *privKey != "" {
		if err := signer.AddHexKey(*privKey); err != nil {
			log.Fatal(err)
		}
		log.Infof("faucet address is %s", signer.AddressString())
	} else {
		if err := signer.Generate(); err != nil {
			log.Fatal(err)
		}
		log.Infof("generated new signing private key %x", signer.PrivateKey())
		log.Warnf("please send VOC tokens to %s", signer.AddressString())
	}

	// init HTTP router
	var httpRouter httprouter.HTTProuter
	httpRouter.TLSdomain = *tlsDomain
	httpRouter.TLSdirCert = *dataDir
	if err := httpRouter.Init(*listenHost, *listenPort); err != nil {
		log.Fatal(err)
	}

	// init storage
	storage, err := newStorage(*dbType, *dataDir, *waitPeriod)
	if err != nil {
		log.Fatal(err)
	}

	// create the faucet instance
	f := faucet{
		signer:     &signer,
		authTypes:  authTypes,
		waitPeriod: *waitPeriod,
		storage:    storage,
	}

	// init API
	api, err := apirest.NewAPI(&httpRouter, *baseRoute)
	if err != nil {
		log.Fatal(err)
	}

	// register handlers
	f.registerHandlers(api)

	log.Infof("API available at %s", *baseRoute)
	log.Info("startup complete")
	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warnf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	os.Exit(0)
}
