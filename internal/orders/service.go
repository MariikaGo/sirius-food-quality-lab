package orders

import (
	"context"
	"sirius.local/food/internal/domain"
)

type Tx interface {
	User(context.Context, int64) (domain.User, error)
	Item(context.Context, int64) (domain.Item, error)
	Order(context.Context, int64) (domain.Order, error)
	Insert(context.Context, domain.Order) (domain.Order, error)
	Status(context.Context, int64, string) error
	Balance(context.Context, int64, domain.Money) error
}
type Store interface {
	Transaction(context.Context, func(Tx) error) error
	ByID(context.Context, int64) (domain.Order, error)
	All(context.Context) ([]domain.Order, error)
}
type Notifier interface {
	AfterOrder(context.Context, int64) error
}
type Service struct {
	Repo     Store
	Notifier Notifier
}

func (s Service) Create(ctx context.Context, user, item, quantity int64) (domain.Order, error) {
	var order domain.Order
	if quantity <= 0 || quantity > 100 {
		return order, domain.Fail("invalid", "Количество должно быть от 1 до 100")
	}
	e := s.Repo.Transaction(ctx, func(tx Tx) error {
		u, e := tx.User(ctx, user)
		if e != nil {
			return e
		}
		if u.Blocked {
			return domain.Fail("forbidden", "Пользователь заблокирован")
		}
		i, e := tx.Item(ctx, item)
		if e != nil {
			return e
		}
		if !i.Available {
			return domain.Fail("invalid", "Блюдо недоступно")
		}
		total := i.Price * domain.Money(quantity)
		if total > u.Balance {
			return domain.Fail("invalid", "Недостаточно средств")
		}
		if e = tx.Balance(ctx, user, u.Balance-total); e != nil {
			return e
		}
		order, e = tx.Insert(ctx, domain.Order{UserID: user, ItemID: item, Quantity: quantity, Total: total, Status: "processing"})
		return e
	})
	if e != nil {
		return order, e
	}
	if s.Notifier != nil {
		if e = s.Notifier.AfterOrder(ctx, order.ID); e != nil {
			return order, e
		}
	}
	return order, nil
}
func (s Service) Get(ctx context.Context, id int64, actor domain.User) (domain.Order, error) {
	o, e := s.Repo.ByID(ctx, id)
	if e == nil {
		e = domain.Owner(o.UserID, actor)
	}
	return o, e
}
func (s Service) Cancel(ctx context.Context, id int64, actor domain.User) error {
	return s.Repo.Transaction(ctx, func(tx Tx) error {
		o, e := tx.Order(ctx, id)
		if e != nil {
			return e
		}
		if e = domain.Owner(o.UserID, actor); e != nil {
			return e
		}
		if o.Status != "processing" {
			return domain.Fail("invalid", "Отменить можно только заказ в обработке")
		}
		u, e := tx.User(ctx, o.UserID)
		if e != nil {
			return e
		}
		if e = tx.Balance(ctx, u.ID, u.Balance+o.Total); e != nil {
			return e
		}
		return tx.Status(ctx, id, "cancelled")
	})
}
func (s Service) Paid(ctx context.Context, id int64) error {
	return s.Repo.Transaction(ctx, func(tx Tx) error {
		o, e := tx.Order(ctx, id)
		if e != nil {
			return e
		}
		if o.Status != "processing" {
			return domain.Fail("invalid", "Заказ не в обработке")
		}
		return tx.Status(ctx, id, "paid")
	})
}
