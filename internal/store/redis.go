package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis wraps redis client.
type Redis struct {
	Client *redis.Client
}

// NewRedis connects to redis with short timeouts.
func NewRedis(addr string) *Redis {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})
	return &Redis{Client: client}
}

// Healthy verifies redis connectivity.
func (r *Redis) Healthy(ctx context.Context) bool {
	if r == nil || r.Client == nil {
		return false
	}
	return r.Client.Ping(ctx).Err() == nil
}
