package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const clickQueue = "queue:clicks"

type CacheLink struct {
	LongURL   string     `json:"long_url"`
	IsActive  bool       `json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCache(cfg Config) *Cache {
	return &Cache{
		rdb: redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		}),
		ttl: cfg.CacheTTL,
	}
}

func (c *Cache) Close() error {
	return c.rdb.Close()
}

func (c *Cache) GetLink(ctx context.Context, code string) (CacheLink, bool, error) {
	v, err := c.rdb.Get(ctx, "link:"+code).Result()
	if err == redis.Nil {
		return CacheLink{}, false, nil
	}
	if err != nil {
		return CacheLink{}, false, err
	}
	var l CacheLink
	if err := json.Unmarshal([]byte(v), &l); err != nil {
		return CacheLink{}, false, err
	}
	return l, true, nil
}

func (c *Cache) SetLink(ctx context.Context, code string, l CacheLink) error {
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	ttl := c.ttl
	if l.ExpiresAt != nil {
		if d := time.Until(*l.ExpiresAt); d > 0 && d < ttl {
			ttl = d
		}
	}
	return c.rdb.Set(ctx, "link:"+code, b, ttl).Err()
}

func (c *Cache) DeleteLink(ctx context.Context, code string) error {
	return c.rdb.Del(ctx, "link:"+code).Err()
}

func (c *Cache) EnqueueClick(ctx context.Context, code string, t time.Time) error {
	payload := fmt.Sprintf("%s|%s", code, t.UTC().Format(time.RFC3339))
	return c.rdb.LPush(ctx, clickQueue, payload).Err()
}

func (c *Cache) PopClicks(ctx context.Context, batch int) ([]string, error) {
	vals, err := c.rdb.RPopCount(ctx, clickQueue, batch).Result()
	if err == redis.Nil {
		return nil, nil
	}
	return vals, err
}

