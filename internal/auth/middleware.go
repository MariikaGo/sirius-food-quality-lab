package auth

import (
	"context"
	"github.com/redis/go-redis/v9"
	"net"
	"net/http"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/transport"
	"strings"
	"time"
)

type Guard struct {
	Users    UserStore
	Security *Security
	Redis    *redis.Client
}

func (g Guard) Protect(next transport.Handler, roles ...string) http.HandlerFunc {
	return transport.Handle(func(w http.ResponseWriter, r *http.Request) error {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		id, version, e := g.Security.Decode(raw)
		if e != nil {
			return e
		}
		u, e := g.Users.ByID(r.Context(), id)
		if e != nil || u.Version != version {
			return domain.Fail("unauthorized", "Сессия недействительна")
		}
		if u.Blocked {
			return domain.Fail("forbidden", "Пользователь заблокирован")
		}
		if len(roles) > 0 {
			if e = domain.Role(u, roles...); e != nil {
				return e
			}
		}
		return next(w, r.WithContext(transport.WithActor(r.Context(), u)))
	})
}

var rateScript = redis.NewScript(`local n=redis.call('INCR',KEYS[1]);if n==1 then redis.call('EXPIRE',KEYS[1],ARGV[1]) end;return n`)

func (g Guard) Limit(next transport.Handler, max int) http.HandlerFunc {
	return transport.Handle(func(w http.ResponseWriter, r *http.Request) error {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := "sirius:go:rate:" + host + ":" + r.URL.Path
		n, e := rateScript.Run(r.Context(), g.Redis, []string{key}, 60).Int()
		if e != nil {
			return domain.Fail("unavailable", "Защита входа недоступна")
		}
		if n > max {
			return domain.Fail("limited", "Слишком много попыток")
		}
		return next(w, r)
	})
}
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if origin != "http://127.0.0.1:8000" && origin != "http://localhost:8000" {
				transport.Error(w, domain.Fail("forbidden", "Origin не разрешён"))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var _ context.Context
var _ = time.Second
