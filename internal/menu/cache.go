package menu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"sirius.local/food/internal/domain"
	"time"
)

type Cached struct {
	Store   Store
	Redis   *redis.Client
	Enabled bool
}
type cacheItem struct {
	ID         int64
	Name       string
	PriceCents int64
	Available  bool
	CategoryID int64
}
type snapshot struct {
	Version int
	Items   []cacheItem
}

func key(category int64) string {
	if category == 0 {
		return "menu:all"
	}
	return fmt.Sprintf("menu:category:%d", category)
}
func (c *Cached) read(ctx context.Context, k string) ([]domain.Item, bool, error) {
	b, e := c.Redis.Get(ctx, k).Bytes()
	if errors.Is(e, redis.Nil) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	var value snapshot
	if e = json.Unmarshal(b, &value); e != nil || value.Version != 1 {
		c.Redis.Del(ctx, k)
		return nil, false, nil
	}
	out := make([]domain.Item, 0, len(value.Items))
	for _, v := range value.Items {
		out = append(out, domain.Item{ID: v.ID, Name: v.Name, Price: domain.Money(v.PriceCents), Available: v.Available, CategoryID: v.CategoryID})
	}
	return out, true, nil
}

var release = redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end;return 0`)

func (c *Cached) lock(ctx context.Context) (func(), error) {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	wait, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		ok, e := c.Redis.SetNX(wait, "menu:lock", token, 30*time.Second).Result()
		if e != nil {
			return nil, e
		}
		if ok {
			return func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				release.Run(ctx, c.Redis, []string{"menu:lock"}, token)
			}, nil
		}
		select {
		case <-wait.Done():
			return nil, domain.Fail("unavailable", "Обновление кеша заняло слишком много времени")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func (c *Cached) All(ctx context.Context, category int64) ([]domain.Item, error) {
	if !c.Enabled {
		return c.Store.All(ctx, category)
	}
	k := key(category)
	out, hit, e := c.read(ctx, k)
	if e != nil {
		log.Printf("Redis fallback: %v", e)
		return c.Store.All(ctx, category)
	}
	if hit {
		return out, nil
	}
	unlock, e := c.lock(ctx)
	if e != nil {
		var de *domain.Error
		if errors.As(e, &de) {
			return nil, e
		}
		return c.Store.All(ctx, category)
	}
	defer unlock()
	out, hit, e = c.read(ctx, k)
	if e != nil {
		return c.Store.All(ctx, category)
	}
	if hit {
		return out, nil
	}
	out, e = c.Store.All(ctx, category)
	if e != nil {
		return nil, e
	}
	value := snapshot{Version: 1, Items: make([]cacheItem, 0, len(out))}
	for _, v := range out {
		value.Items = append(value.Items, cacheItem{v.ID, v.Name, int64(v.Price), v.Available, v.CategoryID})
	}
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	ttl := 300 * time.Second
	if category != 0 {
		ttl = 600 * time.Second
	}
	if e = c.Redis.Set(ctx, k, b, ttl).Err(); e != nil {
		log.Printf("cache fill failed: %v", e)
	}
	return out, nil
}
func (c *Cached) ByID(ctx context.Context, id int64) (domain.Item, error) {
	return c.Store.ByID(ctx, id)
}
func (c *Cached) write(ctx context.Context, operation func() (domain.Item, error), oldCategory func() (int64, error)) (domain.Item, error) {
	if !c.Enabled {
		return operation()
	}
	unlock, e := c.lock(ctx)
	if e != nil {
		var de *domain.Error
		if errors.As(e, &de) {
			return domain.Item{}, e
		}
		log.Printf("Redis unavailable during menu write: %v", e)
		return operation()
	}
	defer unlock()
	old, e := oldCategory()
	if e != nil {
		return domain.Item{}, e
	}
	i, e := operation()
	if e != nil {
		return i, e
	}
	if e = c.Redis.Del(ctx, "menu:all", key(old), key(i.CategoryID)).Err(); e != nil {
		log.Printf("cache invalidation failed; TTL limits stale data: %v", e)
	}
	return i, nil
}
func (c *Cached) Create(ctx context.Context, i domain.Item) (domain.Item, error) {
	return c.write(ctx, func() (domain.Item, error) { return c.Store.Create(ctx, i) }, func() (int64, error) { return i.CategoryID, nil })
}
func (c *Cached) Update(ctx context.Context, id int64, u Update) (domain.Item, error) {
	return c.write(ctx, func() (domain.Item, error) { return c.Store.Update(ctx, id, u) }, func() (int64, error) { i, e := c.Store.ByID(ctx, id); return i.CategoryID, e })
}
