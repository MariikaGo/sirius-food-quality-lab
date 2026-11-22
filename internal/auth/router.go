package auth

import (
	"net/http"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/transport"
)

type Handler struct{ Service Service }

func (h Handler) Login(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if e := r.ParseForm(); e != nil {
		return domain.Fail("invalid", "Некорректная форма")
	}
	p, e := h.Service.Login(r.Context(), r.FormValue("username"), r.FormValue("password"))
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, p)
}
func (h Handler) Refresh(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token string `json:"refresh_token"`
	}
	if e := transport.Decode(w, r, &in); e != nil {
		return e
	}
	p, e := h.Service.Refresh(r.Context(), in.Token)
	if e != nil {
		return e
	}
	return transport.JSON(w, 200, p)
}
