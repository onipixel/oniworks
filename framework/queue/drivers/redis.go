package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onipixel/oniworks/framework/queue"
)

const deadQueue = "oni:queue:dead"

// Redis is a Redis-backed queue driver using sorted sets for delayed jobs
// and lists for ready jobs.
type Redis struct {
	client *redis.Client
}

// NewRedis creates a Redis queue driver.
//
//	d := drivers.NewRedis("redis://localhost:6379")
func NewRedis(redisURL string) (*Redis, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("queue/redis: parse url: %w", err)
	}
	return &Redis{client: redis.NewClient(opt)}, nil
}

// NewRedisFromClient creates a Redis queue driver from an existing client.
func NewRedisFromClient(c *redis.Client) *Redis {
	return &Redis{client: c}
}

func redisKey(q string) string  { return "oni:queue:" + q }
func delayKey(q string) string  { return "oni:queue:" + q + ":delayed" }

func (r *Redis) Push(ctx context.Context, q string, p *queue.Payload) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	if p.AvailableAt.After(time.Now()) {
		// Delayed job → sorted set, score = unix timestamp
		score := float64(p.AvailableAt.Unix())
		return r.client.ZAdd(ctx, delayKey(q), redis.Z{Score: score, Member: string(data)}).Err()
	}

	return r.client.LPush(ctx, redisKey(q), string(data)).Err()
}

func (r *Redis) Pop(ctx context.Context, q string) (*queue.Payload, error) {
	// First: promote any delayed jobs that are now ready
	r.promoteDelayed(ctx, q)

	// RPOP (FIFO)
	val, err := r.client.RPop(ctx, redisKey(q)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var p queue.Payload
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Redis) promoteDelayed(ctx context.Context, q string) {
	now := float64(time.Now().Unix())
	items, err := r.client.ZRangeByScore(ctx, delayKey(q), &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	if err != nil || len(items) == 0 {
		return
	}
	pipe := r.client.Pipeline()
	for _, item := range items {
		pipe.ZRem(ctx, delayKey(q), item)
		pipe.LPush(ctx, redisKey(q), item)
	}
	_, _ = pipe.Exec(ctx)
}

func (r *Redis) Dead(ctx context.Context, p *queue.Payload) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return r.client.LPush(ctx, deadQueue, string(data)).Err()
}

func (r *Redis) Release(ctx context.Context, p *queue.Payload, delay time.Duration) error {
	p.AvailableAt = time.Now().Add(delay)
	return r.Push(ctx, p.Queue, p)
}

// DeadLetters returns up to n dead-lettered payloads.
func (r *Redis) DeadLetters(ctx context.Context, n int64) ([]*queue.Payload, error) {
	items, err := r.client.LRange(ctx, deadQueue, 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*queue.Payload, 0, len(items))
	for _, item := range items {
		var p queue.Payload
		if err := json.Unmarshal([]byte(item), &p); err == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}
