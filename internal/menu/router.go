package menu

import (
	"net/http"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/transport"
)

type Handler struct{ Service Service }

func (h Handler) All(w http.ResponseWriter, r *http.Request) error {
	var category int64
	if r.PathValue("id") != "" {
		var e error
		category, e = transport.PathID(r, "id")
		if e != nil {
			return e
		}
	}
	items, e := h.Service.All(r.Context(), category)
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, items)
}
func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name       string       `json:"name"`
		Price      domain.Money `json:"price"`
		CategoryID int64        `json:"category_id"`
	}
	if e := transport.Decode(w, r, &in); e != nil {
		return e
	}
	i, e := h.Service.Create(r.Context(), domain.Item{Name: in.Name, Price: in.Price, CategoryID: in.CategoryID})
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]any{"status": "success", "item_id": i.ID})
}
func (h Handler) Update(w http.ResponseWriter, r *http.Request) error {
	id, e := transport.PathID(r, "id")
	if e != nil {
		return e
	}
	var u Update
	if e = transport.Decode(w, r, &u); e != nil {
		return e
	}
	i, e := h.Service.Update(r.Context(), id, u)
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, map[string]any{"status": "success", "item_id": i.ID})
}
