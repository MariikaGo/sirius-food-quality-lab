package delivery

import (
	"context"
	"sirius.local/food/internal/domain"
)

type Tx interface {
	Order(context.Context, int64) (domain.Order, error)
	Courier(context.Context, int64) (domain.User, error)
	Active(context.Context, int64) (bool, error)
	Insert(context.Context, int64, int64) (domain.Delivery, error)
	Delivery(context.Context, int64) (domain.Delivery, error)
	Complete(context.Context, domain.Delivery) error
}
type Store interface {
	Transaction(context.Context, func(Tx) error) error
	ByOrder(context.Context, int64) (domain.Delivery, error)
	Owner(context.Context, int64) (int64, error)
}
type Gateway interface {
	Assign(context.Context, int64, int64, *domain.User) (domain.Delivery, error)
	Get(context.Context, int64, domain.User) (domain.Delivery, error)
	Complete(context.Context, int64, *domain.User) error
}
type Service struct {
	Repo   Store
	Secure bool
}

func (s Service) Assign(ctx context.Context, order, courier int64, actor *domain.User) (domain.Delivery, error) {
	var d domain.Delivery
	if s.Secure {
		if actor == nil {
			return d, domain.Fail("unauthorized", "Нужен токен")
		}
		if e := domain.Role(*actor, "admin", "cook"); e != nil {
			return d, e
		}
	}
	e := s.Repo.Transaction(ctx, func(tx Tx) error {
		o, e := tx.Order(ctx, order)
		if e != nil {
			return e
		}
		if o.Status != "paid" {
			return domain.Fail("invalid", "Курьера можно назначить только на оплаченный заказ")
		}
		u, e := tx.Courier(ctx, courier)
		if e != nil {
			return e
		}
		if u.Blocked {
			return domain.Fail("forbidden", "Курьер заблокирован")
		}
		if s.Secure && u.Role != "courier" {
			return domain.Fail("invalid", "Нужна роль courier")
		}
		active, e := tx.Active(ctx, courier)
		if e != nil {
			return e
		}
		if active {
			return domain.Fail("invalid", "У курьера уже есть активная доставка")
		}
		d, e = tx.Insert(ctx, order, courier)
		return e
	})
	return d, e
}
func (s Service) Get(ctx context.Context, order int64, actor domain.User) (domain.Delivery, error) {
	d, e := s.Repo.ByOrder(ctx, order)
	if e != nil {
		return d, e
	}
	if actor.Role == "courier" {
		e = domain.Owner(d.CourierID, actor)
	} else {
		var owner int64
		owner, e = s.Repo.Owner(ctx, order)
		if e == nil {
			e = domain.Owner(owner, actor)
		}
	}
	return d, e
}
func (s Service) Complete(ctx context.Context, id int64, actor *domain.User) error {
	return s.Repo.Transaction(ctx, func(tx Tx) error {
		d, e := tx.Delivery(ctx, id)
		if e != nil {
			return e
		}
		if s.Secure {
			if actor == nil {
				return domain.Fail("unauthorized", "Нужен токен")
			}
			if e = domain.Role(*actor, "admin", "courier"); e != nil {
				return e
			}
			if e = domain.Owner(d.CourierID, *actor); e != nil {
				return e
			}
		}
		if d.Status != "in_progress" {
			return domain.Fail("invalid", "Доставка уже завершена или отменена")
		}
		return tx.Complete(ctx, d)
	})
}
