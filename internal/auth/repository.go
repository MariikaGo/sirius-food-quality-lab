package auth

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/users"
	"time"
)

type Repository struct{ Pool *pgxpool.Pool }

func (r *Repository) Save(ctx context.Context, id int64, hash string, expires time.Time) error {
	_, e := r.Pool.Exec(ctx, "INSERT INTO refresh_tokens(user_id,token_hash,expires_at)VALUES($1,$2,$3)", id, hash, expires)
	return e
}
func (r *Repository) Rotate(ctx context.Context, old, next string, expires time.Time) (domain.User, error) {
	bad := domain.Fail("unauthorized", "Refresh-токен недействителен или повторно использован")
	var zero domain.User
	tx, e := r.Pool.Begin(ctx)
	if e != nil {
		return zero, e
	}
	defer tx.Rollback(ctx)
	var uid int64
	if e = tx.QueryRow(ctx, "SELECT user_id FROM refresh_tokens WHERE token_hash=$1", old).Scan(&uid); e != nil {
		return zero, bad
	}
	ur := users.Repository{}
	u, e := users.Scan(tx.QueryRow(ctx, "SELECT "+ur.Columns()+" FROM users WHERE id=$1 FOR UPDATE", uid))
	if e != nil {
		return u, e
	}
	var revoked bool
	var expiry time.Time
	if e = tx.QueryRow(ctx, "SELECT is_revoked,expires_at FROM refresh_tokens WHERE token_hash=$1 FOR UPDATE", old).Scan(&revoked, &expiry); e != nil {
		return u, bad
	}
	if revoked {
		if _, e = tx.Exec(ctx, "UPDATE refresh_tokens SET is_revoked=true WHERE user_id=$1", uid); e != nil {
			return u, e
		}
		if _, e = tx.Exec(ctx, "UPDATE users SET session_version=session_version+1 WHERE id=$1", uid); e != nil {
			return u, e
		}
		if e = tx.Commit(ctx); e != nil {
			return u, e
		}
		return u, bad
	}
	if u.Blocked || time.Now().After(expiry) {
		return u, bad
	}
	if _, e = tx.Exec(ctx, "UPDATE refresh_tokens SET is_revoked=true WHERE token_hash=$1", old); e != nil {
		return u, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO refresh_tokens(user_id,token_hash,expires_at)VALUES($1,$2,$3)", uid, next, expires); e != nil {
		return u, e
	}
	return u, tx.Commit(ctx)
}
