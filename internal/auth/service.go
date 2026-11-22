package auth

import (
	"context"
	"sirius.local/food/internal/domain"
	"strings"
	"time"
)

type UserStore interface {
	ByID(context.Context, int64) (domain.User, error)
	ByEmail(context.Context, string) (domain.User, error)
}
type TokenStore interface {
	Save(context.Context, int64, string, time.Time) error
	Rotate(context.Context, string, string, time.Time) (domain.User, error)
}
type Passwords interface {
	Verify(string, string) bool
	Access(domain.User) (string, error)
}
type Service struct {
	Users    UserStore
	Tokens   TokenStore
	Security Passwords
}
type Pair struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	Type    string `json:"token_type"`
}

func (s Service) pair(u domain.User, refresh string) (Pair, error) {
	access, e := s.Security.Access(u)
	return Pair{access, refresh, "bearer"}, e
}
func (s Service) Login(ctx context.Context, email, password string) (Pair, error) {
	u, e := s.Users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if e != nil || !s.Security.Verify(password, u.Hash) {
		return Pair{}, domain.Fail("unauthorized", "Неверный email или пароль")
	}
	if u.Blocked {
		return Pair{}, domain.Fail("forbidden", "Пользователь заблокирован")
	}
	token := Random()
	if e = s.Tokens.Save(ctx, u.ID, HashToken(token), time.Now().Add(7*24*time.Hour)); e != nil {
		return Pair{}, e
	}
	return s.pair(u, token)
}
func (s Service) Refresh(ctx context.Context, old string) (Pair, error) {
	if len(old) < 20 || len(old) > 200 {
		return Pair{}, domain.Fail("unauthorized", "Недействительный refresh")
	}
	token := Random()
	u, e := s.Tokens.Rotate(ctx, HashToken(old), HashToken(token), time.Now().Add(7*24*time.Hour))
	if e != nil {
		return Pair{}, e
	}
	return s.pair(u, token)
}
