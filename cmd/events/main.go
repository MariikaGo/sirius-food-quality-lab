package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/eda"
	"sirius.local/food/internal/observe"
	"syscall"
)

func main() {
	shutdown := observe.Install("events")
	defer shutdown()
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	if len(os.Args) != 2 {
		log.Fatal("use publisher/kitchen/delivery/compensation")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load()
	p, e := database.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	eda.Run(ctx, p, os.Args[1])
}
