package users

import (
	"context"
	"sirius.local/food/internal/domain"
	"testing"
)

func TestServiceWithMemoryRepository(t *testing.T) {
	s := Service{Repo: NewMemory()}
	ctx := context.Background()
	u, e := s.Register(ctx, "unit@example.com", "", "")
	if e != nil {
		t.Fatal(e)
	}
	u, e = s.Topup(ctx, u.ID, 10010, nil)
	if e != nil || u.Balance != 10010 {
		t.Fatalf("%+v %v", u, e)
	}
	if _, e = s.Topup(ctx, u.ID, 5000001, nil); e == nil {
		t.Fatal("accepted over limit")
	}
	other := domain.User{ID: 999, Role: "student"}
	if _, e = s.Topup(ctx, u.ID, 100, &other); e == nil {
		t.Fatal("foreign object accepted")
	}
}
