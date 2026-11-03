package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/patterniha/PattNG/tor-core/bridge"
	"github.com/patterniha/PattNG/tor-core/config"
	"github.com/patterniha/PattNG/tor-core/engine"
	"github.com/patterniha/PattNG/tor-core/transport"
)

func main() {
	log.SetFlags(0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if transport.IsManagedPTEnv() {
		reg := transport.NewRegistry(bridge.Toggles{
			Snowflake: true,
			Lyrebird:  true,
			Conjure:   true,
		})
		if err := transport.RunManagedClient(ctx, reg); err != nil {
			log.Fatalf("[-] tor-core managed pt error: %v", err)
		}
		return
	}

	cfg, err := config.ParseFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("[-] tor-core configuration error: %v", err)
	}

	if err := engine.New(cfg).Run(ctx); err != nil {
		log.Fatalf("[-] tor-core failed: %v", err)
	}
}
