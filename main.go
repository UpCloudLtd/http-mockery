package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/ajmyyra/http-mockery/pkg/mockery"
)

const (
	configEnvName     = "HTTP_MOCKERY_CONFIG"
	defaultConfigFile = "config.json"
)

func main() {
	configFile := defaultConfigFile
	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	}
	if envConfig, found := os.LookupEnv(configEnvName); found {
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

	listenAddr := fmt.Sprintf("%s:%d", config.ListenIP, config.ListenPort)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal("Unable to start listener at ", listenAddr, ": ", err.Error())
	}

	h := mockery.MockHandler{
		Config: config,
	}

	if err = h.ValidateConfig(); err != nil {
		log.Fatal("Config validation failed: ", err.Error())
	}

	log.Fatal(http.Serve(l, h))
}
