package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"notification-service/internal/models"

	"github.com/redis/go-redis/v9"
)

const (
	StatusCacheTTL = 10 * time.Minute
	DedupCacheTTL  = 5 * time.Minute
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (r *RedisCache) CacheStatus(ctx context.Context, id string, n *models.Notification) error {
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, fmt.Sprintf("notif:status:%s", id), data, StatusCacheTTL).Err()
}

func (r *RedisCache) GetCachedStatus(ctx context.Context, id string) (*models.Notification, error) {
	data, err := r.client.Get(ctx, fmt.Sprintf("notif:status:%s", id)).Result()
	if err != nil {
		return nil, err
	}
	var n models.Notification
	if err := json.Unmarshal([]byte(data), &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *RedisCache) InvalidateStatus(ctx context.Context, id string) error {
	return r.client.Del(ctx, fmt.Sprintf("notif:status:%s", id)).Err()
}

// SetDedup marks a notification as recently sent (for duplicate suppression)
func (r *RedisCache) SetDedup(ctx context.Context, recipient string, nType models.NotificationType) error {
	key := fmt.Sprintf("notif:dedup:%s:%s", recipient, nType)
	return r.client.Set(ctx, key, "1", DedupCacheTTL).Err()
}

// CheckDedup returns true if a duplicate exists in cache
func (r *RedisCache) CheckDedup(ctx context.Context, recipient string, nType models.NotificationType) bool {
	key := fmt.Sprintf("notif:dedup:%s:%s", recipient, nType)
	val, err := r.client.Exists(ctx, key).Result()
	return err == nil && val > 0
}
