package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/observe"
	"sirius.local/food/internal/rpcdelivery"
	"syscall"
	"time"
)

func main() {
	shutdown := observe.Install("delivery")
	defer shutdown()
	cfg := config.Load()
	if len(cfg.ServiceToken) < 32 {
		log.Fatal("set SERVICE_TOKEN with at least 32 bytes")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	p, e := database.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	l, e := net.Listen("tcp", cfg.GRPCAddr)
	if e != nil {
		log.Fatal(e)
	}
	srv := rpcdelivery.NewServer(p, cfg)
	go func() {
		<-ctx.Done()
		done := make(chan struct{})
		go func() { srv.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			srv.Stop()
		}
	}()
	log.Printf("Go delivery gRPC listening on %s", cfg.GRPCAddr)
	if e = srv.Serve(l); e != nil {
		log.Fatal(e)
	}
}
