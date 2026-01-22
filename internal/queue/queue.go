package queue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message represents work to be processed.
type Message struct {
	Type string
	Body []byte
}

// Queue is the abstraction over different backends.
type Queue interface {
	Publish(ctx context.Context, msg Message) error
	Consume(ctx context.Context) (<-chan Message, error)
}

// InMemory is a minimal channel-backed queue for dev/testing.
type InMemory struct {
	ch chan Message
}

// NewInMemory creates a bounded in-memory queue.
func NewInMemory(size int) *InMemory {
	return &InMemory{ch: make(chan Message, size)}
}

// Publish enqueues a message.
func (q *InMemory) Publish(ctx context.Context, msg Message) error {
	select {
	case q.ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Consume returns a channel for workers.
func (q *InMemory) Consume(ctx context.Context) (<-chan Message, error) {
	out := make(chan Message)
	go func() {
		defer close(out)
		for {
			select {
			case msg := <-q.ch:
				out <- msg
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// RedisQueue implements a simple Redis list-backed queue.
type RedisQueue struct {
	client *redis.Client
	key    string
}

// NewRedisQueue builds a queue using LPUSH/BRPOP semantics.
func NewRedisQueue(client *redis.Client, key string) *RedisQueue {
	if key == "" {
		key = "attendance:queue"
	}
	return &RedisQueue{client: client, key: key}
}

// Publish enqueues a message.
func (q *RedisQueue) Publish(ctx context.Context, msg Message) error {
	return q.client.LPush(ctx, q.key, serialize(msg)).Err()
}

// Consume streams messages using BRPOP.
func (q *RedisQueue) Consume(ctx context.Context) (<-chan Message, error) {
	out := make(chan Message)
	go func() {
		defer close(out)
		for {
			res, err := q.client.BRPop(ctx, 5*time.Second, q.key).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				continue
			}
			if len(res) == 2 {
				if msg, err := deserialize(res[1]); err == nil {
					out <- msg
				}
			}
		}
	}()
	return out, nil
}

// serialize is a tiny helper to store messages as Type|Body.
func serialize(msg Message) string {
	return msg.Type + "|" + string(msg.Body)
}

func deserialize(s string) (Message, error) {
	parts := []rune(s)
	for i, r := range parts {
		if r == '|' {
			return Message{Type: string(parts[:i]), Body: []byte(string(parts[i+1:]))}, nil
		}
	}
	return Message{Body: []byte(s)}, nil
}
