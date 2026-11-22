package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sirius.local/food/internal/domain"
)

type Handler func(http.ResponseWriter, *http.Request) error

func Handle(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := h(w, r); e != nil {
			Error(w, e)
		}
	}
}
func JSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return domain.Fail("invalid", "Некорректный JSON: "+e.Error())
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return domain.Fail("invalid", "Ожидается один JSON-объект")
	}
	return nil
}
func Error(w http.ResponseWriter, e error) {
	status := 500
	message := "Внутренняя ошибка"
	var de *domain.Error
	if errors.As(e, &de) {
		message = de.Message
		status = map[string]int{"invalid": 400, "unauthorized": 401, "forbidden": 403, "not_found": 404, "conflict": 409, "limited": 429, "unavailable": 503, "deadline": 504}[de.Code]
		if status == 0 {
			status = 500
		}
	} else {
		log.Printf("request error: %v", e)
	}
	if status == 401 {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	if status == 429 {
		w.Header().Set("Retry-After", "60")
	}
	JSON(w, status, map[string]string{"detail": message})
}

type actorKey struct{}

func WithActor(ctx context.Context, u domain.User) context.Context {
	return context.WithValue(ctx, actorKey{}, u)
}
func Actor(r *http.Request) *domain.User {
	u, ok := r.Context().Value(actorKey{}).(domain.User)
	if !ok {
		return nil
	}
	return &u
}
func PathID(r *http.Request, name string) (int64, error) { return domain.ID(r.PathValue(name)) }
