package users

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"sirius.local/food/internal/domain"
)

type Repository struct {
	Pool *pgxpool.Pool
}

func (r *Repository) Columns() string {

	return "id,email,phone,balance_cents,is_blocked,role,password_hash,session_version"
}
func Scan(row pgx.Row) (domain.User, error) {
	var u domain.User
	e := row.Scan(&u.ID, &u.Email, &u.Phone, &u.Balance, &u.Blocked, &u.Role, &u.Hash, &u.Version)
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.Fail("not_found", "Пользователь не найден")
	}
	return u, e
}
func (r *Repository) ByID(ctx context.Context, id int64) (domain.User, error) {
	return Scan(r.Pool.QueryRow(ctx, "SELECT "+r.Columns()+" FROM users WHERE id=$1", id))
}
func (r *Repository) ByEmail(ctx context.Context, email string) (domain.User, error) {
	return Scan(r.Pool.QueryRow(ctx, "SELECT "+r.Columns()+" FROM users WHERE email=$1", email))
}
func (r *Repository) Create(ctx context.Context, email, phone, hash string) (domain.User, error) {
	query := "INSERT INTO users(email,phone,password_hash,role)VALUES($1,$2,$3,'student') RETURNING " + r.Columns()
	args := []any{email, phone, hash}
	u, e := Scan(r.Pool.QueryRow(ctx, query, args...))
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		e = domain.Fail("conflict", "Email уже зарегистрирован")
	}
	return u, e
}
func (r *Repository) Topup(ctx context.Context, id int64, amount domain.Money) (domain.User, error) {
	tx, e := r.Pool.Begin(ctx)
	if e != nil {
		return domain.User{}, e
	}
	defer tx.Rollback(ctx)
	u, e := Scan(tx.QueryRow(ctx, "SELECT "+r.Columns()+" FROM users WHERE id=$1 FOR UPDATE", id))
	if e != nil {
		return u, e
	}
	if u.Blocked {
		return u, domain.Fail("forbidden", "Пользователь заблокирован")
	}
	u.Balance += amount
	_, e = tx.Exec(ctx, "UPDATE users SET balance_cents=$2 WHERE id=$1", id, u.Balance)
	if e == nil {
		e = tx.Commit(ctx)
	}
	return u, e
}
