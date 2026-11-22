package orders

import (
	"context"
	"sirius.local/food/internal/domain"
	"testing"
)

type fakeStore struct {
	user  domain.User
	item  domain.Item
	order domain.Order
}

func (f *fakeStore) Transaction(ctx context.Context, fn func(Tx) error) error { return fn(f) }
func (f *fakeStore) ByID(context.Context, int64) (domain.Order, error)        { return f.order, nil }
func (f *fakeStore) All(context.Context) ([]domain.Order, error)              { return []domain.Order{f.order}, nil }
func (f *fakeStore) User(context.Context, int64) (domain.User, error)         { return f.user, nil }
func (f *fakeStore) Item(context.Context, int64) (domain.Item, error)         { return f.item, nil }
func (f *fakeStore) Order(context.Context, int64) (domain.Order, error)       { return f.order, nil }
func (f *fakeStore) Insert(_ context.Context, o domain.Order) (domain.Order, error) {
	o.ID = 1
	f.order = o
	return o, nil
}
func (f *fakeStore) Status(_ context.Context, _ int64, s string) error {
	f.order.Status = s
	return nil
}
func (f *fakeStore) Balance(_ context.Context, _ int64, v domain.Money) error {
	f.user.Balance = v
	return nil
}
func TestCreateBusinessBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		quantity                 int64
		balance                  domain.Money
		available, blocked, want bool
	}{{"zero", 0, 15000, true, false, false}, {"negative", -1, 15000, true, false, false}, {"too_many", 101, 9999999, true, false, false}, {"exact_balance", 1, 15000, true, false, true}, {"insufficient", 1, 14999, true, false, false}, {"unavailable", 1, 15000, false, false, false}, {"blocked", 1, 15000, true, true, false}, {"normal", 2, 40000, true, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeStore{user: domain.User{ID: 1, Balance: tc.balance, Blocked: tc.blocked}, item: domain.Item{ID: 1, Price: 15000, Available: tc.available}}
			s := Service{Repo: f}
			o, e := s.Create(context.Background(), 1, 1, tc.quantity)
			if (e == nil) != tc.want {
				t.Fatalf("success=%v want=%v error=%v", e == nil, tc.want, e)
			}
			if tc.want && (o.Total != 15000*domain.Money(tc.quantity) || f.user.Balance != tc.balance-o.Total) {
				t.Fatal("incorrect total or debit")
			}
		})
	}
}
func TestCancelRefundOnce(t *testing.T) {
	f := &fakeStore{user: domain.User{ID: 1, Balance: 10000}, order: domain.Order{ID: 1, UserID: 1, Total: 15000, Status: "processing"}}
	s := Service{Repo: f}
	if e := s.Cancel(context.Background(), 1, f.user); e != nil {
		t.Fatal(e)
	}
	if e := s.Cancel(context.Background(), 1, f.user); e == nil {
		t.Fatal("second refund accepted")
	}
	if f.user.Balance != 25000 {
		t.Fatal(f.user.Balance)
	}
}
