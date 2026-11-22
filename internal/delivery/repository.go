package delivery

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/users"
)

type Repository struct {
	Pool *pgxpool.Pool
}
type sqlTx struct {
	pgx.Tx
	r *Repository
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
func (t sqlTx) Order(ctx context.Context, id int64) (domain.Order, error) {
	var o domain.Order
	e := t.QueryRow(ctx, "SELECT id,status FROM orders WHERE id=$1 FOR UPDATE", id).Scan(&o.ID, &o.Status)
	return o, notfound(e)
}
func (t sqlTx) Courier(ctx context.Context, id int64) (domain.User, error) {
	r := users.Repository{}
	return users.Scan(t.QueryRow(ctx, "SELECT "+r.Columns()+" FROM users WHERE id=$1 FOR UPDATE", id))
}
func (t sqlTx) Active(ctx context.Context, id int64) (bool, error) {
	var active bool
	e := t.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM deliveries WHERE courier_id=$1 AND status='in_progress')", id).Scan(&active)
	return active, e
}
func notfound(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Fail("not_found", "Заказ или доставка не найдены")
	}
	return e
}
func scan(row pgx.Row) (domain.Delivery, error) {
	var d domain.Delivery
	e := row.Scan(&d.ID, &d.OrderID, &d.CourierID, &d.Status)
	return d, notfound(e)
}
func (t sqlTx) Insert(ctx context.Context, order, courier int64) (domain.Delivery, error) {
	d, e := scan(t.QueryRow(ctx, "INSERT INTO deliveries(order_id,courier_id)VALUES($1,$2)RETURNING id,order_id,courier_id,status", order, courier))
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		e = domain.Fail("conflict", "Доставка или активное назначение уже существует")
	}
	return d, e
}
func (t sqlTx) Delivery(ctx context.Context, id int64) (domain.Delivery, error) {
	return scan(t.QueryRow(ctx, "SELECT id,order_id,courier_id,status FROM deliveries WHERE id=$1 FOR UPDATE", id))
}
func (t sqlTx) Complete(ctx context.Context, d domain.Delivery) error {
	if _, e := t.Exec(ctx, "UPDATE deliveries SET status='completed'WHERE id=$1", d.ID); e != nil {
		return e
	}
	_, e := t.Exec(ctx, "UPDATE orders SET status='delivered'WHERE id=$1", d.OrderID)
	return e
}
func (r *Repository) ByOrder(ctx context.Context, id int64) (domain.Delivery, error) {
	return scan(r.Pool.QueryRow(ctx, "SELECT id,order_id,courier_id,status FROM deliveries WHERE order_id=$1", id))
}
func (r *Repository) Owner(ctx context.Context, id int64) (int64, error) {
	var owner int64
	e := r.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id=$1", id).Scan(&owner)
	return owner, notfound(e)
}
