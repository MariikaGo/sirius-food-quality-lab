package delivery

import (
	"net/http"
	"sirius.local/food/internal/transport"
)

type Handler struct{ Service Gateway }

func (h Handler) Assign(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		OrderID   int64 `json:"order_id"`
		CourierID int64 `json:"courier_id"`
	}
	if e := transport.Decode(w, r, &in); e != nil {
		return e
	}
	d, e := h.Service.Assign(r.Context(), in.OrderID, in.CourierID, transport.Actor(r))
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]any{"status": "success", "delivery_id": d.ID})
}
func (h Handler) Get(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	d, e := h.Service.Get(r.Context(), id, *transport.Actor(r))
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, d)
}
func (h Handler) Complete(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	if e = h.Service.Complete(r.Context(), id, transport.Actor(r)); e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]string{"status": "success"})
}
