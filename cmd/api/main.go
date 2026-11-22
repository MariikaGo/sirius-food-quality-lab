package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sirius.local/food/internal/app"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/observe"
	"syscall"
	"time"
)

func main() {
	shutdown := observe.Install("api")
	defer shutdown()
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	p, e := database.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	if e = database.Migrate(ctx, p, config.Stage); e != nil {
		log.Fatal(e)
	}
	handler, closeFn, e := app.New(ctx, p, cfg)
	if e != nil {
		log.Fatal(e)
	}
	defer closeFn()
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(c)
	}()
	log.Printf("Sirius.Food practice %d: http://%s", config.Stage, cfg.HTTPAddr)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
