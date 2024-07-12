package main

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/vocdoni/vocfaucet/faucet"
	"github.com/vocdoni/vocfaucet/storage"
	"github.com/vocdoni/vocfaucet/stripehandler"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

var supportedAuthTypes = map[string]string{
	"open":      "without authentication, anyone can use the faucet",
	"oauth":     "with oauth2 authentication",
	"aragondao": "signed message from addresses belonging to at least one aragon dao",
	"stripe":    "with stripe payment",
}

func main() {
	flag.String("logLevel", "info", "log level")
	flag.String("tlsDomain", "", "domain for tls (implies listen to port 443)")
	flag.String("listenHost", "0.0.0.0", "host to listen on")
	flag.Int("listenPort", 8080, "port to listen on")
	flag.String("baseRoute", "/v2", "base route for the API")
	flag.String("dataDir", "./vocfaucet-data", "data directory")
	flag.String("privKey", "", "private key for the faucet signer (hexadecimal)")
	flag.String("auth", "open", "authentication types to use (comma separated): open, oauth")
	flag.String("amounts", "100", "tokens to send per request (comma separated), the order must match the auth types")
	flag.Duration("waitPeriod", 1*time.Hour, "wait period between requests for the same user")
	flag.StringP("dbType", "t", db.TypePebble, fmt.Sprintf("key-value db type [%s,%s,%s]", db.TypePebble, db.TypeLevelDB, db.TypeMongo))
	flag.String("stripeKey", "", "stripe secret key")
	flag.String("stripePriceId", "", "stripe price id")
	flag.Int64("stripeMinQuantity", 100, "stripe min number of tokens")
	flag.Int64("stripeMaxQuantity", 100000, "stripe max number of tokens")
	flag.String("stripeWebhookSecret", "", "stripe webhook secret key")
	flag.Parse()

	// Setting up viper
	viper := viper.New()
	viper.SetConfigName("faucet")
	viper.SetConfigType("yml")
	viper.SetEnvPrefix("")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set FlagVars first
	if err := viper.BindPFlag("dataDir", flag.Lookup("dataDir")); err != nil {
		panic(err)
	}
	dataDir := path.Clean(viper.GetString("dataDir"))
	viper.AddConfigPath(dataDir)
	fmt.Printf("Using path %s\n", dataDir)
	if err := viper.BindPFlag("logLevel", flag.Lookup("logLevel")); err != nil {
		panic(err)
	}
	logLevel := viper.GetString("logLevel")
	log.Init(logLevel, "stdout", nil)
	if err := viper.BindPFlag("tlsDomain", flag.Lookup("tlsDomain")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("listenHost", flag.Lookup("listenHost")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("listenPort", flag.Lookup("listenPort")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("baseRoute", flag.Lookup("baseRoute")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("privKey", flag.Lookup("privKey")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("auth", flag.Lookup("auth")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("amounts", flag.Lookup("amounts")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("waitPeriod", flag.Lookup("waitPeriod")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("dbType", flag.Lookup("dbType")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("stripeKey", flag.Lookup("stripeKey")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("stripePriceId", flag.Lookup("stripePriceId")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("stripeMinQuantity", flag.Lookup("stripeMinQuantity")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("stripeMaxQuantity", flag.Lookup("stripeMaxQuantity")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("stripeWebhookSecret", flag.Lookup("stripeWebhookSecret")); err != nil {
		panic(err)
	}

	// check if config file exists
	_, err := os.Stat(path.Join(dataDir, "faucet.yml"))
	if os.IsNotExist(err) {
		fmt.Printf("creating new config file in %s\n", dataDir)
		// creting config folder if not exists
		err = os.MkdirAll(dataDir, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("cannot create data directory: %v", err))
		}
		// create config file if not exists
		if err := viper.SafeWriteConfig(); err != nil {
			panic(fmt.Sprintf("cannot write config file into config dir: %v", err))
		}

	} else {
		// read config file
		err = viper.ReadInConfig()
		if err != nil {
			panic(fmt.Sprintf("cannot read loaded config file in %s: %v", dataDir, err))
		}
	}
	// save config file
	if err := viper.WriteConfig(); err != nil {
		panic(fmt.Sprintf("cannot write config file into config dir: %v", err))
	}

	// Set Viper/Flag variables
	tlsDomain := viper.GetString("tlsDomain")
	listenHost := viper.GetString("listenHost")
	listenPort := viper.GetInt("listenPort")
	baseRoute := viper.GetString("baseRoute")
	privKey := viper.GetString("privKey")
	auth := viper.GetString("auth")
	amounts := viper.GetString("amounts")

	waitPeriod := viper.GetDuration("waitPeriod")
	dbType := viper.GetString("dbType")
	stripeKey := viper.GetString("stripeKey")
	stripePriceId := viper.GetString("stripePriceId")
	stripeMinQuantity := viper.GetInt64("stripeMinQuantity")
	stripeMaxQuantity := viper.GetInt64("stripeMaxQuantity")
	stripeWebhookSecret := viper.GetString("stripeWebhookSecret")

	// parse auth types and amounts
	authNames := strings.Split(auth, ",")
	for _, t := range authNames {
		if _, ok := supportedAuthTypes[t]; !ok {
			log.Fatalf("unsupported authentication type %s", t)
		}
	}
	amountsStr := strings.Split(amounts, ",")
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
	if privKey != "" {
		if err := signer.AddHexKey(privKey); err != nil {
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
	httpRouter.TLSdomain = tlsDomain
	httpRouter.TLSdirCert = dataDir
	if err := httpRouter.Init(listenHost, listenPort); err != nil {
		log.Fatal(err)
	}

	// init storage
	storage, err := storage.New(dbType, dataDir, waitPeriod, signer.Address().Bytes()[:8])
	if err != nil {
		log.Fatal(err)
	}
	// create the faucet instance
	f := faucet.Faucet{
		Signer:     &signer,
		AuthTypes:  authTypes,
		WaitPeriod: waitPeriod,
		Storage:    storage,
	}
	var s *stripehandler.StripeHandler
	if amount := f.AuthTypes[faucet.AuthTypeStripe]; amount > 0 {
		s, err = stripehandler.NewStripeClient(
			stripeKey,
			stripePriceId,
			stripeWebhookSecret,
			stripeMinQuantity,
			stripeMaxQuantity,
			int64(amount),
			&f,
			storage,
		)
		if err != nil {
			log.Fatalf("stripe initialization error: %s", err)
		} else {
			log.Infof("stripe enabled with price id %s", stripePriceId)
		}
	}

	// init API
	api, err := apirest.NewAPI(&httpRouter, baseRoute)
	if err != nil {
		log.Fatal(err)
	}

	// register handlers
	f.RegisterHandlers(api)
	s.RegisterHandlers(api)
	log.Infof("API available at %s", baseRoute)
	log.Info("startup complete")
	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warnf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	os.Exit(0)
}
