package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Seed(ctx context.Context, p *pgxpool.Pool, stage int, hash func(string) (string, error)) error {
	for _, name := range []string{"student", "student2", "cook", "courier", "courier2", "admin"} {

		var exists bool
		if e := p.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)", name+"@example.com").Scan(&exists); e != nil {
			return e
		}
		if exists {
			continue
		}
		password, e := hash("SiriusDemo2026!")
		if e != nil {
			return e
		}
		role := name
		if name == "student2" {
			role = "student"
		}
		if name == "courier2" {
			role = "courier"
		}
		if _, e = p.Exec(ctx, "INSERT INTO users(email,password_hash,role,balance_cents)VALUES($1,$2,$3,1000000)", name+"@example.com", password, role); e != nil {
			return e
		}

	}
	for i, name := range []string{"Борщ", "Плов", "Чай"} {
		if _, e := p.Exec(ctx, "INSERT INTO menu_items(name,price_cents,category_id)VALUES($1,$2,$3)ON CONFLICT(name)DO NOTHING", name, []int64{15000, 25000, 5000}[i], i+1); e != nil {
			return e
		}
	}
	fmt.Println("Демо-данные готовы; пароль: SiriusDemo2026!")
	return nil
}
