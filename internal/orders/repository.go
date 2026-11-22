package orders

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/eda"
	"sirius.local/food/internal/users"
)

type Repository struct {
	Pool *pgxpool.Pool

	Jobs bool
}
type sqlTx struct {
	pgx.Tx
	repo *Repository
}

func (r *Repository) Transaction(ctx context.Context, f func(Tx) error) error {
	tx, e := r.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = f(sqlTx{tx, r}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (t sqlTx) User(ctx context.Context, id int64) (domain.User, error) {
	u := users.Repository{}
	return users.Scan(t.QueryRow(ctx, "SELECT "+u.Columns()+" FROM users WHERE id=$1 FOR UPDATE", id))
}
func (t sqlTx) Item(ctx context.Context, id int64) (domain.Item, error) {
	var i domain.Item
	e := t.QueryRow(ctx, "SELECT id,name,price_cents,is_available,category_id FROM menu_items WHERE id=$1 FOR SHARE", id).Scan(&i.ID, &i.Name, &i.Price, &i.Available, &i.CategoryID)
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.Fail("not_found", "Блюдо не найдено")
	}
	return i, e
}

const cols = "id,user_id,item_id,quantity,status,total_cents"

func scan(row pgx.Row) (domain.Order, error) {
	var o domain.Order
	e := row.Scan(&o.ID, &o.UserID, &o.ItemID, &o.Quantity, &o.Status, &o.Total)
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.Fail("not_found", "Заказ не найден")
	}
	return o, e
}
func (t sqlTx) Order(ctx context.Context, id int64) (domain.Order, error) {
	return scan(t.QueryRow(ctx, "SELECT "+cols+" FROM orders WHERE id=$1 FOR UPDATE", id))
}
func (t sqlTx) Insert(ctx context.Context, o domain.Order) (domain.Order, error) {
	o, e := scan(t.QueryRow(ctx, "INSERT INTO orders(user_id,item_id,quantity,status,total_cents)VALUES($1,$2,$3,$4,$5)RETURNING "+cols, o.UserID, o.ItemID, o.Quantity, o.Status, o.Total))
	if e == nil && t.repo.Jobs {
		for _, kind := range []string{"pdf", "email", "sms"} {
			if _, e = t.Exec(ctx, "INSERT INTO task_logs(order_id,kind)VALUES($1,$2)", o.ID, kind); e != nil {
				break
			}
		}
	}
	return o, e
}
func (t sqlTx) Balance(ctx context.Context, id int64, m domain.Money) error {
	_, e := t.Exec(ctx, "UPDATE users SET balance_cents=$2 WHERE id=$1", id, m)
	return e
}
func (t sqlTx) Status(ctx context.Context, id int64, status string) error {
	_, e := t.Exec(ctx, "UPDATE orders SET status=$2 WHERE id=$1", id, status)
	if e == nil && status == "paid" {
		e = eda.Record(ctx, t.Tx, "order.paid", id)
	}
	return e
}
func (r *Repository) ByID(ctx context.Context, id int64) (domain.Order, error) {
	return scan(r.Pool.QueryRow(ctx, "SELECT "+cols+" FROM orders WHERE id=$1", id))
}
func (r *Repository) All(ctx context.Context) ([]domain.Order, error) {
	rows, e := r.Pool.Query(ctx, "SELECT "+cols+" FROM orders ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Order{}
	for rows.Next() {
		o, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
