package users

import (
	"context"
	"net/mail"
	"sirius.local/food/internal/domain"
	"strings"
)

type Store interface {
	ByID(context.Context, int64) (domain.User, error)
	ByEmail(context.Context, string) (domain.User, error)
	Create(context.Context, string, string, string) (domain.User, error)
	Topup(context.Context, int64, domain.Money) (domain.User, error)
}
type Hasher interface{ Hash(string) (string, error) }
type Service struct {
	Repo   Store
	Hasher Hasher
}

func (s Service) Register(ctx context.Context, email, phone, password string) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email {
		return domain.User{}, domain.Fail("invalid", "Некорректный email")
	}
	if phone != "" && len(phone) < 10 {
		return domain.User{}, domain.Fail("invalid", "Некорректный телефон")
	}
	hash := ""
	if s.Hasher != nil {
		if len(password) < 10 || len(password) > 72 {
			return domain.User{}, domain.Fail("invalid", "Пароль должен содержать 10–72 байта")
		}
		hash, e = s.Hasher.Hash(password)
		if e != nil {
			return domain.User{}, e
		}
	}
	return s.Repo.Create(ctx, email, phone, hash)
}
func (s Service) Topup(ctx context.Context, id int64, amount domain.Money, actor *domain.User) (domain.User, error) {
	if actor != nil {
		if e := domain.Owner(id, *actor); e != nil {
			return domain.User{}, e
		}
	}
	if amount <= 0 || amount > 5000000 {
		return domain.User{}, domain.Fail("invalid", "Сумма должна быть от 0.01 до 50000 рублей")
	}
	return s.Repo.Topup(ctx, id, amount)
}
