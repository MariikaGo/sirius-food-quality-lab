package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sirius.local/food/internal/app"
	"sirius.local/food/internal/auth"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/rpcdelivery"
	"strings"

	"testing"
)

func setup(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("SIRIUS_TEST_DB")
	if url == "" {
		t.Skip("set SIRIUS_TEST_DB for PostgreSQL integration tests")
	}
	ctx := context.Background()
	p, e := database.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasSuffix(p.Config().ConnConfig.Database, "_test") {
		t.Fatal("test database name must end with _test")
	}
	if _, e = p.Exec(ctx, "DROP TABLE IF EXISTS kitchen_orders,processed_events,event_outbox,task_logs,refresh_tokens,deliveries,orders,menu_items,users,roles,schema_migrations CASCADE"); e != nil {
		t.Fatal(e)
	}
	if e = database.Migrate(ctx, p, config.Stage); e != nil {
		t.Fatal(e)
	}
	t.Setenv("BCRYPT_COST", "4")
	t.Setenv("REDIS_DB", "9")
	cfg := config.Load()
	cfg.JWTSecret = "go-test-secret-longer-than-thirty-two-characters"
	cfg.DatabaseURL = url
	cfg.TasksMode = "disabled"
	t.Setenv("ASYNQ_DB", "11")
	cache := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, DB: 9})
	cache.FlushDB(ctx)
	cache.Close()

	if e = database.Seed(ctx, p, config.Stage, auth.NewSecurity(cfg.JWTSecret).Hash); e != nil {
		t.Fatal(e)
	}

	var rpcServer *grpc.Server

	cfg.ServiceToken = "test-service-token-longer-than-thirty-two-characters"
	t.Setenv("STREAM_INTERVAL_MS", "10")
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	cfg.GRPCAddr = listener.Addr().String()
	rpcServer = rpcdelivery.NewServer(p, cfg)
	go rpcServer.Serve(listener)

	h, closeFn, e := app.New(ctx, p, cfg)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(h)
	t.Cleanup(func() {
		server.Close()
		closeFn()
		if rpcServer != nil {
			rpcServer.Stop()
		}
		p.Close()
	})
	return server, p
}
func request(t *testing.T, s *httptest.Server, method, path string, in any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			t.Fatal(e)
		}
		body = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, s.URL+path, body)
	if in != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	resp, e := s.Client().Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	var out map[string]any
	b, _ := io.ReadAll(resp.Body)
	json.Unmarshal(b, &out)
	return resp.StatusCode, out
}
