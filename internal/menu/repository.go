package menu

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"sirius.local/food/internal/domain"
	"sync/atomic"
)

type Repository struct {
	Pool  *pgxpool.Pool
	Reads atomic.Int64
}

func scan(row pgx.Row) (domain.Item, error) {
	var i domain.Item
	e := row.Scan(&i.ID, &i.Name, &i.Price, &i.Available, &i.CategoryID)
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.Fail("not_found", "Блюдо не найдено")
	}
	return i, e
}

const columns = "id,name,price_cents,is_available,category_id"

func (r *Repository) All(ctx context.Context, category int64) ([]domain.Item, error) {
	r.Reads.Add(1)
	rows, e := r.Pool.Query(ctx, "SELECT "+columns+" FROM menu_items WHERE ($1::bigint=0 OR category_id=$1) ORDER BY id", category)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Item{}
	for rows.Next() {
		i, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
func (r *Repository) ByID(ctx context.Context, id int64) (domain.Item, error) {
	return scan(r.Pool.QueryRow(ctx, "SELECT "+columns+" FROM menu_items WHERE id=$1", id))
}
func unique(e error) error {
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		return domain.Fail("conflict", "Блюдо с таким названием уже существует")
	}
	return e
}
func (r *Repository) Create(ctx context.Context, i domain.Item) (domain.Item, error) {
	v, e := scan(r.Pool.QueryRow(ctx, "INSERT INTO menu_items(name,price_cents,is_available,category_id)VALUES($1,$2,$3,$4) RETURNING "+columns, i.Name, i.Price, i.Available, i.CategoryID))
	return v, unique(e)
}
func (r *Repository) Update(ctx context.Context, id int64, u Update) (domain.Item, error) {
	i, e := scan(r.Pool.QueryRow(ctx, "UPDATE menu_items SET name=COALESCE($2,name),price_cents=COALESCE($3,price_cents),is_available=COALESCE($4,is_available),category_id=COALESCE($5,category_id) WHERE id=$1 RETURNING "+columns, id, u.Name, u.Price, u.Available, u.CategoryID))
	return i, unique(e)
}
