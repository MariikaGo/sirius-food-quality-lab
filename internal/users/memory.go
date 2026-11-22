package users

import (
	"context"
	"sirius.local/food/internal/domain"
	"sync"
)

type Memory struct {
	mu   sync.Mutex
	Rows map[int64]domain.User
}

func NewMemory() *Memory { return &Memory{Rows: map[int64]domain.User{}} }
func (m *Memory) ByID(_ context.Context, id int64) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.Rows[id]
	if !ok {
		return u, domain.Fail("not_found", "Пользователь не найден")
	}
	return u, nil
}
func (m *Memory) ByEmail(_ context.Context, email string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.Rows {
		if u.Email == email {
			return u, nil
		}
	}
	return domain.User{}, domain.Fail("not_found", "Пользователь не найден")
}
func (m *Memory) Create(_ context.Context, email, phone, hash string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.Rows {
		if u.Email == email {
			return u, domain.Fail("conflict", "Email уже зарегистрирован")
		}
	}
	u := domain.User{ID: int64(len(m.Rows) + 1), Email: email, Phone: phone, Hash: hash, Role: "student"}
	m.Rows[u.ID] = u
	return u, nil
}
func (m *Memory) Topup(_ context.Context, id int64, amount domain.Money) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.Rows[id]
	if !ok {
		return u, domain.Fail("not_found", "Пользователь не найден")
	}
	if u.Blocked {
		return u, domain.Fail("forbidden", "Пользователь заблокирован")
	}
	u.Balance += amount
	m.Rows[id] = u
	return u, nil
}
