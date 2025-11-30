package clients

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient will handle communication with Redis instances
type RedisClient struct {
	// No need for a connection pool here, as each call will create a new client
	// for a specific address. For a real production system, a connection pool
	// or persistent client for each known Redis instance might be preferred.
}

// NewRedisClient creates a new Redis client
func NewRedisClient() *RedisClient {
	return &RedisClient{}
}

// newRedisUniversalClient creates a new redis.UniversalClient for a given address.
// It's designed to be used for one-off commands or short-lived interactions.
func (c *RedisClient) newRedisUniversalClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "", // no password set
		DB:           0,  // use default DB
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     1, // Small pool for supervisor's needs
	})
}

// GetRole checks the replication role of a Redis node (master or slave)
func (c *RedisClient) GetRole(addr string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rdb := c.newRedisUniversalClient(addr)
	defer rdb.Close()

	info, err := rdb.Info(ctx, "replication").Result()
	if err != nil {
		return "", fmt.Errorf("failed to get INFO replication from %s: %w", addr, err)
	}

	// Parse the info string to find the role
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("could not determine role from Redis INFO replication for %s", addr)
}

// Ping checks the health of a Redis node
func (c *RedisClient) Ping(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	rdb := c.newRedisUniversalClient(addr)
	defer rdb.Close()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to PING %s: %w", addr, err)
	}
	return nil
}

// PromoteToPrimary sends the 'REPLICAOF NO ONE' command to turn a replica into a primary
func (c *RedisClient) PromoteToPrimary(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rdb := c.newRedisUniversalClient(addr)
	defer rdb.Close()

	// Send SLAVEOF NO ONE to promote it to primary
	_, err := rdb.SlaveOf(ctx, "no", "one").Result()
	if err != nil {
		return fmt.Errorf("failed to promote %s to primary: %w", addr, err)
	}
	return nil
}
