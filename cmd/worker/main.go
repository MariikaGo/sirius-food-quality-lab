package main

import (
	"context"
	"log"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/jobs"
)

func main() {
	cfg := config.Load()
	p, e := database.Open(context.Background(), cfg.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	engine := jobs.New(p, cfg)
	defer engine.Close()
	srv, mux := engine.Worker()
	log.Print("Go Asynq worker started")
	if e = srv.Run(mux); e != nil {
		log.Fatal(e)
	}
}
