package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/whinis/external-dns-vegadns-webhook/cmd/webhook/init/configuration"
	"github.com/whinis/external-dns-vegadns-webhook/cmd/webhook/init/dnsprovider"
	"github.com/whinis/external-dns-vegadns-webhook/cmd/webhook/init/logging"
	"github.com/whinis/external-dns-vegadns-webhook/cmd/webhook/init/server"
	"github.com/whinis/external-dns-vegadns-webhook/pkg/webhook"
)

const banner = `
     ______________ 
    / ____/  _/ __ \
   / __/  / // /_/ /
  / /____/ // ____/ 
 /_____/___/_/      
 external-dns-VegasDNS-webhook
 version: %s (%s)
`

var (
	// Version - value can be overridden by ldflags
	Version = "local"
	Gitsha  = "?"
)

func main() {
	fmt.Printf(banner, Version, Gitsha)

	logging.Init()

	config := configuration.Init()
	provider, err := dnsprovider.Init(config)
	if err != nil {
		log.Fatalf("failed to initialize provider: %v", err)
	}
	srv := server.Init(config, webhook.New(provider))
	server.ShutdownGracefully(srv)
}
