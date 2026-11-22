package domain

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Money хранит целое число копеек. JSON API принимает и возвращает рубли.
type Money int64

func (m Money) MarshalJSON() ([]byte, error) {
	negative := ""
	v := int64(m)
	if v < 0 {
		negative = "-"
		v = -v
	}
	return []byte(fmt.Sprintf("%s%d.%02d", negative, v/100, v%100)), nil
}
func (m *Money) UnmarshalJSON(b []byte) error {
	v, e := ParseMoney(string(b))
	if e == nil {
		*m = v
	}
	return e
}
func ParseMoney(s string) (Money, error) {
	r, ok := new(big.Rat).SetString(strings.Trim(s, "\""))
	if !ok {
		return 0, fmt.Errorf("некорректная сумма")
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, fmt.Errorf("не более двух десятичных знаков")
	}
	v := r.Num().Int64()
	if v > 100000000000000 || v < -100000000000000 {
		return 0, fmt.Errorf("сумма слишком велика")
	}
	return Money(v), nil
}

type User struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Balance Money  `json:"balance"`
	Blocked bool   `json:"is_blocked"`
	Role    string `json:"role"`
	Hash    string `json:"-"`
	Version int64  `json:"-"`
}
type Item struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Price      Money  `json:"price"`
	Available  bool   `json:"is_available"`
	CategoryID int64  `json:"category_id"`
}
type Order struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	ItemID   int64  `json:"item_id"`
	Quantity int64  `json:"quantity"`
	Status   string `json:"status"`
	Total    Money  `json:"total"`
}
type Delivery struct {
	ID        int64  `json:"id"`
	OrderID   int64  `json:"order_id"`
	CourierID int64  `json:"courier_id"`
	Status    string `json:"status"`
}
type Error struct{ Code, Message string }

func (e *Error) Error() string        { return e.Message }
func Fail(code, message string) error { return &Error{code, message} }
func Owner(owner int64, u User) error {
	if u.Role != "admin" && u.ID != owner {
		return Fail("forbidden", "Нет доступа к чужому объекту")
	}
	return nil
}
func Role(u User, roles ...string) error {
	for _, r := range roles {
		if u.Role == r {
			return nil
		}
	}
	return Fail("forbidden", "Недостаточно прав")
}
func ID(s string) (int64, error) {
	id, e := strconv.ParseInt(s, 10, 64)
	if e != nil || id <= 0 {
		return 0, Fail("invalid", "Нужен положительный ID")
	}
	return id, nil
}

var _ json.Marshaler = Money(0)
