package orders

import (
	"net/http"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/transport"
)

type Handler struct {
	Service Service
	Secure  bool
}

func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		UserID   int64 `json:"user_id"`
		ItemID   int64 `json:"item_id"`
		Quantity int64 `json:"quantity"`
	}
	if e := transport.Decode(w, r, &in); e != nil {
		return e
	}
	if h.Secure {
		if in.UserID != 0 {
			return domain.Fail("invalid", "user_id определяется по токену")
		}
		in.UserID = transport.Actor(r).ID
	}
	o, e := h.Service.Create(r.Context(), in.UserID, in.ItemID, in.Quantity)
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]any{"status": "success", "order_id": o.ID})
}
func (h Handler) Get(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	o, e := h.Service.Get(r.Context(), id, *transport.Actor(r))
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, o)
}
func (h Handler) Cancel(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	if e = h.Service.Cancel(r.Context(), id, *transport.Actor(r)); e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]string{"status": "success"})
}
func (h Handler) Paid(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	if e = h.Service.Paid(r.Context(), id); e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]string{"status": "success"})
}
func (h Handler) Kitchen(w http.ResponseWriter, r *http.Request) error {
	o, e := h.Service.Repo.All(r.Context())
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, o)
}
