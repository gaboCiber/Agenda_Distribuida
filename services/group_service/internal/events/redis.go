package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

// RedisClient wraps the Redis client
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisClient creates a new Redis client
func NewRedisClient(url string) *RedisClient {
	// Parse Redis URL
	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Fatalf("Invalid Redis URL: %v", err)
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Connected to Redis")
	return &RedisClient{
		client: client,
		ctx:    ctx,
	}
}

// Publish publishes a message to a channel
func (r *RedisClient) Publish(channel string, message interface{}) error {
	// Convert message to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// Publish message
	result := r.client.Publish(r.ctx, channel, payload)
	if result.Err() != nil {
		return fmt.Errorf("failed to publish message: %v", result.Err())
	}

	log.Printf("📤 Published to %s: %s", channel, payload)
	return nil
}

// Subscribe subscribes to a channel and calls the handler for each message
func (r *RedisClient) Subscribe(channel string, handler func(string)) error {
	// Create a new pubsub connection
	pubsub := r.client.Subscribe(r.ctx, channel)

	// Close the subscription when we're done
	defer pubsub.Close()

	// Get the channel to receive messages
	ch := pubsub.Channel()

	// Process messages
	for msg := range ch {
		log.Printf("📥 Received message on %s: %s", channel, msg.Payload)
		go handler(msg.Payload)
	}

	return nil
}

// Close closes the Redis client
func (r *RedisClient) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
