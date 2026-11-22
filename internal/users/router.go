package users

import (
	"net/http"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/transport"
)

type Handler struct{ Service Service }

func (h Handler) Register(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if e := transport.Decode(w, r, &in); e != nil {
		return e
	}
	u, e := h.Service.Register(r.Context(), in.Email, in.Phone, in.Password)
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]any{"status": "success", "user_id": u.ID, "email": u.Email})
}
func (h Handler) Topup(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	amount, e := domain.ParseMoney(r.URL.Query().Get("amount"))
	if e != nil {
		return domain.Fail("invalid", e.Error())
	}
	u, e := h.Service.Topup(r.Context(), id, amount, transport.Actor(r))
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]any{"status": "success", "new_balance": u.Balance})
}
func (h Handler) Me(w http.ResponseWriter, r *http.Request) error {
	return transport.JSON(w, 200, transport.Actor(r))
}
