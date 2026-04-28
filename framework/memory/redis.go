package memory

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const gossipChannel = "oni:memory:sync"

// redisSyncAdapter uses Redis pub/sub as the cross-node sync transport.
// Use this instead of the built-in gossip when Redis is already in your stack.
type redisSyncAdapter struct {
	store  *Store
	client *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
}

type redisDelta struct {
	Type      string          `json:"t"`
	Key       string          `json:"k,omitempty"`
	Value     json.RawMessage `json:"v,omitempty"`
	ExpiresAt int64           `json:"exp,omitempty"` // unix nano
	Topic     string          `json:"tp,omitempty"`
}

func newRedisSyncAdapter(store *Store, redisURL string) *redisSyncAdapter {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("memory: invalid Redis URL", "url", redisURL, "error", err)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &redisSyncAdapter{
		store:  store,
		client: redis.NewClient(opt),
		ctx:    ctx,
		cancel: cancel,
		logger: slog.Default(),
	}
	go a.subscribe()
	return a
}

func (a *redisSyncAdapter) subscribe() {
	pubsub := a.client.Subscribe(a.ctx, gossipChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()

	for {
		select {
		case <-a.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			a.applyMsg(msg.Payload)
		}
	}
}

func (a *redisSyncAdapter) applyMsg(raw string) {
	var delta redisDelta
	if err := json.Unmarshal([]byte(raw), &delta); err != nil {
		a.logger.Warn("memory: redis sync: bad message", "error", err)
		return
	}
	switch delta.Type {
	case "set":
		var value any
		if err := json.Unmarshal(delta.Value, &value); err != nil {
			return
		}
		var exp time.Time
		if delta.ExpiresAt > 0 {
			exp = time.Unix(0, delta.ExpiresAt)
		}
		a.store.applyRemoteSet(delta.Key, value, exp, ClockValue{})
	case "delete":
		a.store.applyRemoteDelete(delta.Key)
	case "publish":
		var value any
		if err := json.Unmarshal(delta.Value, &value); err != nil {
			return
		}
		a.store.applyRemotePublish(delta.Topic, value)
	}
}

func (a *redisSyncAdapter) publish(delta redisDelta) {
	b, err := json.Marshal(delta)
	if err != nil {
		return
	}
	if err := a.client.Publish(a.ctx, gossipChannel, string(b)).Err(); err != nil {
		a.logger.Warn("memory: redis publish failed", "error", err)
	}
}

func (a *redisSyncAdapter) broadcastSet(key string, e *entry) {
	b, _ := json.Marshal(e.Value)
	var exp int64
	if !e.ExpiresAt.IsZero() {
		exp = e.ExpiresAt.UnixNano()
	}
	a.publish(redisDelta{Type: "set", Key: key, Value: json.RawMessage(b), ExpiresAt: exp})
}

func (a *redisSyncAdapter) broadcastDelete(key string) {
	a.publish(redisDelta{Type: "delete", Key: key})
}

func (a *redisSyncAdapter) broadcastPublish(topic string, payload any) {
	b, _ := json.Marshal(payload)
	a.publish(redisDelta{Type: "publish", Topic: topic, Value: json.RawMessage(b)})
}

func (a *redisSyncAdapter) stop() {
	a.cancel()
	_ = a.client.Close()
}
