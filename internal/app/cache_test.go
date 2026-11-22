package app_test

import (
	"context"
	"github.com/redis/go-redis/v9"
	"net/http"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/menu"
	"sync"
	"testing"
	"time"
)

func TestCacheWarmupTTLInvalidation(t *testing.T) {

	s, _ := setup(t)
	ctx := context.Background()
	cache := redis.NewClient(&redis.Options{Addr: config.Load().RedisAddr, DB: 9})
	defer cache.Close()
	ttl := cache.TTL(ctx, "menu:all").Val()
	if ttl <= 0 || ttl > 300*time.Second {
		t.Fatal(ttl)
	}
	r, e := s.Client().Get(s.URL + "/menu/categories/1")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	ttl = cache.TTL(ctx, "menu:category:1").Val()
	if ttl <= 0 || ttl > 600*time.Second {
		t.Fatal(ttl)
	}
	r, _ = s.Client().Get(s.URL + "/menu/categories/2")
	r.Body.Close()
	h, _ := login(t, s, "cook")
	code, out := request(t, s, http.MethodPatch, "/menu/items/1", map[string]any{"category_id": 2, "name": "Обновленный борщ"}, h)
	if code != 200 {
		t.Fatal(code, out)
	}
	if cache.Exists(ctx, "menu:all", "menu:category:1", "menu:category:2").Val() != 0 {
		t.Fatal("cache not invalidated")
	}
}
func TestCacheStampede(t *testing.T) {

	_, p := setup(t)
	ctx := context.Background()
	cache := redis.NewClient(&redis.Options{Addr: config.Load().RedisAddr, DB: 9})
	defer cache.Close()
	cache.Del(ctx, "menu:all")
	repo := &menu.Repository{Pool: p}
	cached := &menu.Cached{Store: repo, Redis: cache, Enabled: true}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for range 40 {
		wg.Go(func() { _, e := cached.All(ctx, 0); errs <- e })
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if repo.Reads.Load() != 1 {
		t.Fatal("expected one SQL read", repo.Reads.Load())
	}
}
func TestRedisFallback(t *testing.T) {

	_, p := setup(t)
	cache := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 20 * time.Millisecond})
	defer cache.Close()
	cached := &menu.Cached{Store: &menu.Repository{Pool: p}, Redis: cache, Enabled: true}
	rows, e := cached.All(context.Background(), 0)
	if e != nil || len(rows) != 3 {
		t.Fatal(len(rows), e)
	}
}
