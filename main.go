package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/UpCloudLtd/http-mockery/pkg/mockery"
)

const (
	EnvVarConfigFile          = "HTTP_MOCKERY_CONFIG"
	EnvVarLogRequestContents  = "HTTP_MOCKERY_REQUEST_CONTENTS"
	EnvVarLogResponseContents = "HTTP_MOCKERY_RESPONSE_CONTENTS"
	defaultConfigFile         = "config.json"
)

func main() {
	configFile := defaultConfigFile
	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	}
	if envConfig, found := os.LookupEnv(EnvVarConfigFile); found {
		configFile = envConfig
	}

	config, err := mockery.OpenConfigFile(configFile)
	if err != nil {
		log.Fatal("Unable to open config file ", configFile, ": ", err.Error())
	}

	if config.ListenIP == "" {
		config.ListenIP = "0.0.0.0"
	}
	if config.ListenPort == 0 {
		config.ListenPort = 8080
	}

	if _, found := os.LookupEnv(EnvVarLogRequestContents); found {
		config.Logging.RequestContents = true
	}
	if _, found := os.LookupEnv(EnvVarLogResponseContents); found {
		config.Logging.ResponseContents = true
	}

	listenAddr := fmt.Sprintf("%s:%d", config.ListenIP, config.ListenPort)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal("Unable to start listener at ", listenAddr, ": ", err.Error())
	}

	h := mockery.MockHandler{
		Config: config,
		Log:    log.Default(),
	}

	if err = h.ValidateConfig(); err != nil {
		log.Fatal("Config validation failed: ", err.Error())
	}

	h.Log.Printf("start listening %s", listenAddr)
	h.Log.Fatal(http.Serve(l, h))
}
