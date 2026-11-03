package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/patterniha/PattNG/tor-core/conjure"
)

func main() {
	log.SetFlags(0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := conjure.RunManagedClient(ctx); err != nil {
		log.Fatalf("[-] conjure-client error: %v", err)
	}
}
