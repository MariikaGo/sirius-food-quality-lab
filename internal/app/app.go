package app

import (
	"context"
	"fmt"
	"github.com/hibiken/asynqmon"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"net/http"
	"sirius.local/food/internal/auth"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/delivery"
	"sirius.local/food/internal/docs"
	"sirius.local/food/internal/jobs"
	"sirius.local/food/internal/menu"
	"sirius.local/food/internal/observe"
	"sirius.local/food/internal/orders"
	"sirius.local/food/internal/rpcdelivery"
	"sirius.local/food/internal/transport"
	"sirius.local/food/internal/users"
)

func New(ctx context.Context, p *pgxpool.Pool, cfg config.Config) (http.Handler, func(), error) {
	if len(cfg.JWTSecret) < 32 {
		return nil, nil, fmt.Errorf("set JWT_SECRET with at least 32 bytes")
	}
	observe.SetPool(p)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", observe.Metrics())
	if cfg.Debug {
		docs.Mount(mux, config.Stage)
	}
	cache := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, DB: config.Int("REDIS_DB", 0), DialTimeout: timeSecond, ReadTimeout: timeSecond, WriteTimeout: timeSecond})
	ur := &users.Repository{Pool: p}
	mr := &menu.Repository{Pool: p}
	or := &orders.Repository{Pool: p, Jobs: cfg.TasksMode != "disabled"}
	rpcClient, e := rpcdelivery.NewClient(cfg)
	if e != nil {
		cache.Close()
		return nil, nil, e
	}
	security := auth.NewSecurity(cfg.JWTSecret)
	guard := auth.Guard{Users: ur, Security: security, Redis: cache}
	ah := auth.Handler{Service: auth.Service{Users: ur, Tokens: &auth.Repository{Pool: p}, Security: security}}
	uh := users.Handler{Service: users.Service{Repo: ur, Hasher: security}}
	menuStore := &menu.Cached{Store: mr, Redis: cache, Enabled: cfg.Cache}
	if cfg.Cache {
		if _, e := menuStore.All(ctx, 0); e != nil {
			cache.Close()
			return nil, nil, e
		}
	}
	mh := menu.Handler{Service: menu.Service{Repo: menuStore}}
	engine := jobs.New(p, cfg)
	oh := orders.Handler{Service: orders.Service{Repo: or, Notifier: engine}, Secure: true}
	closeMonitor := func() {}
	if cfg.Debug {
		monitor := asynqmon.New(asynqmon.Options{RootPath: "/monitor", RedisConnOpt: engine.Opt, ReadOnly: true})
		mux.Handle("/monitor/", monitor)
		mux.Handle("/monitor", monitor)
		closeMonitor = func() { monitor.Close() }
	}
	dh := delivery.Handler{Service: rpcClient}
	mux.HandleFunc("GET /health", transport.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return transport.JSON(w, 200, map[string]any{"status": "ok", "practice": config.Stage})
	}))
	mux.HandleFunc("POST /auth/login", guard.Limit(ah.Login, 5))
	mux.HandleFunc("POST /auth/refresh", guard.Limit(ah.Refresh, 30))
	mux.HandleFunc("POST /users/register", transport.Handle(uh.Register))
	mux.HandleFunc("POST /users/{id}/topup", guard.Protect(uh.Topup))
	mux.HandleFunc("GET /users/me", guard.Protect(uh.Me))
	mux.HandleFunc("GET /menu/items/{id}", transport.Handle(mh.Get))
	mux.HandleFunc("GET /menu/items", transport.Handle(mh.All))
	mux.HandleFunc("GET /menu/categories/{id}", transport.Handle(mh.All))
	mux.HandleFunc("POST /menu/items", guard.Protect(mh.Create, "cook", "admin"))
	mux.HandleFunc("PATCH /menu/items/{id}", guard.Protect(mh.Update, "cook", "admin"))
	mux.HandleFunc("POST /orders/create", guard.Protect(oh.Create))
	mux.HandleFunc("GET /orders/{id}", guard.Protect(oh.Get))
	mux.HandleFunc("POST /orders/{id}/cancel", guard.Protect(oh.Cancel))
	mux.HandleFunc("POST /orders/{id}/paid", guard.Protect(oh.Paid, "cook", "admin"))
	mux.HandleFunc("GET /orders/kitchen/orders", guard.Protect(oh.Kitchen, "cook", "admin"))
	mux.HandleFunc("POST /delivery/assign", guard.Protect(dh.Assign, "cook", "admin"))
	mux.HandleFunc("PATCH /delivery/{id}/complete", guard.Protect(dh.Complete, "courier", "admin"))
	mux.HandleFunc("GET /delivery/orders/{id}", guard.Protect(dh.Get))
	mux.HandleFunc("GET /internal/metrics", guard.Protect(func(w http.ResponseWriter, r *http.Request) error {
		return transport.JSON(w, 200, map[string]int64{"menu_sql_reads": mr.Reads.Load()})
	}, "admin"))
	return observe.HTTP(auth.CORS(mux)), func() { rpcClient.Close(); closeMonitor(); cache.Close(); engine.Close() }, nil
}

const timeSecond = 1000000000
