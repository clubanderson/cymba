package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/kcp-dev/kcp/pkg/server"
)

func main() {
	// Setup signal handler for a cleaner shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()
	srv := server.NewServer(server.DefaultConfig())
	srv.Run(ctx)	
}