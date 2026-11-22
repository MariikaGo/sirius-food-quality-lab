package menu

import (
	"context"
	"sirius.local/food/internal/domain"
	"strings"
)

type Update struct {
	Name       *string       `json:"name"`
	Price      *domain.Money `json:"price"`
	Available  *bool         `json:"is_available"`
	CategoryID *int64        `json:"category_id"`
}
type Store interface {
	All(context.Context, int64) ([]domain.Item, error)
	ByID(context.Context, int64) (domain.Item, error)
	Create(context.Context, domain.Item) (domain.Item, error)
	Update(context.Context, int64, Update) (domain.Item, error)
}
type Service struct{ Repo Store }

func (s Service) All(ctx context.Context, category int64) ([]domain.Item, error) {
	return s.Repo.All(ctx, category)
}
func (s Service) Create(ctx context.Context, i domain.Item) (domain.Item, error) {
	i.Name = strings.TrimSpace(i.Name)
	if len([]rune(i.Name)) < 2 || i.Price <= 0 {
		return i, domain.Fail("invalid", "Нужно название от 2 символов и положительная цена")
	}
	if i.CategoryID <= 0 {
		i.CategoryID = 1
	}
	i.Available = true
	return s.Repo.Create(ctx, i)
}
func (s Service) Update(ctx context.Context, id int64, u Update) (domain.Item, error) {
	if u.Name != nil && len([]rune(strings.TrimSpace(*u.Name))) < 2 || u.Price != nil && *u.Price <= 0 || u.CategoryID != nil && *u.CategoryID <= 0 {
		return domain.Item{}, domain.Fail("invalid", "Некорректные поля блюда")
	}
	return s.Repo.Update(ctx, id, u)
}
