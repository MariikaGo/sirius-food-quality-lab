package main

import (
	"context"
	"log"
	"sirius.local/food/internal/auth"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
)

func main() {
	ctx := context.Background()
	c := config.Load()
	p, e := database.Open(ctx, c.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	if e = database.Migrate(ctx, p, config.Stage); e != nil {
		log.Fatal(e)
	}
	if e = database.Seed(ctx, p, config.Stage, auth.NewSecurity(c.JWTSecret).Hash); e != nil {
		log.Fatal(e)
	}
}
