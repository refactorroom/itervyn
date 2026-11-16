package main

import (
	"context"
	"github.com/akkaraponph/itervyn/internal/cli"
	"github.com/akkaraponph/itervyn/internal/evidence"
	"github.com/akkaraponph/itervyn/internal/providers/evaris"
	"github.com/akkaraponph/itervyn/internal/providers/invaris"
	"github.com/akkaraponph/itervyn/internal/verify"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, verify.Service{Providers: []verify.Provider{invaris.Adapter{}, evaris.Adapter{}}, Store: evidence.Store{}}))
}
