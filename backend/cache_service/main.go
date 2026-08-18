package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// CacheService manages Redis cache for account balances
type CacheService struct {
	rdb *redis.Client
	ctx context.Context
}

// NewCacheService creates a new CacheService instance
func NewCacheService(rdb *redis.Client) *CacheService {
	return &CacheService{
		rdb: rdb,
		ctx: context.Background(),
	}
}

// InvalidateBalance removes balance from cache for given account IDs
func (s *CacheService) InvalidateBalance(accountIDs []string) error {
	for _, id := range accountIDs {
		if err := s.rdb.Del(s.ctx, "balance:"+id).Err(); err != nil {
			return fmt.Errorf("failed to invalidate balance for account %s: %w", id, err)
		}
	}
	return nil
}

// GetBalanceFromCache retrieves balance from cache
func (s *CacheService) GetBalanceFromCache(accountID string) (float64, bool, error) {
	val, err := s.rdb.Get(s.ctx, "balance:"+accountID).Result()
	if err == redis.Nil {
		return 0, false, nil // Not found in cache
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get balance from cache: %w", err)
	}

	var balance float64
	_, err = fmt.Sscanf(val, "%f", &balance)
	if err != nil {
		return 0, false, fmt.Errorf("failed to parse balance from cache: %w", err)
	}

	return balance, true, nil
}

// SetBalanceInCache stores balance in cache
func (s *CacheService) SetBalanceInCache(accountID string, balance float64) error {
	err := s.rdb.Set(s.ctx, "balance:"+accountID, fmt.Sprintf("%f", balance), 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set balance in cache: %w", err)
	}
	return nil
}

// PublishInvalidationEvent publishes balance invalidation event to Redis Stream
func (s *CacheService) PublishInvalidationEvent(accountIDs []string) error {
	invalidationMsg := map[string]interface{}{
		"account_ids": accountIDs,
		"timestamp":   time.Now().Unix(),
	}
	msgJSON, _ := json.Marshal(invalidationMsg)
	_, err := s.rdb.XAdd(s.ctx, &redis.XAddArgs{
		Stream: "balance:invalidation",
		Values: map[string]interface{}{"data": msgJSON},
	}).Result()
	return err
}

// SubscribeToInvalidationEvents listens for balance invalidation events
func (s *CacheService) SubscribeToInvalidationEvents() <-chan []string {
	ch := make(chan []string)

	go func() {
		defer close(ch)
		var lastID string = "0" // Start from the beginning

		for {
			// Block until we get new messages
			results, err := s.rdb.XRead(s.ctx, &redis.XReadArgs{
				Streams: []string{"balance:invalidation", lastID},
				Count:   10,
				Block:   0, // Block indefinitely
			}).Result()

			if err != nil {
				log.Printf("Error reading from Redis Stream: %v", err)
				time.Sleep(1 * time.Second) // Wait before retrying
				continue
			}

			// Process the results
			if len(results) > 0 && len(results[0].Messages) > 0 {
				for _, msg := range results[0].Messages {
					// Update lastID to the latest message ID we processed
					if msg.ID > lastID {
						lastID = msg.ID
					}

					// Parse the invalidation message
					var invalidationMsg struct {
						AccountIDs []string `json:"account_ids"`
						Timestamp  int64    `json:"timestamp"`
					}

					if err := json.Unmarshal([]byte(msg.Message["data"].(string)), &invalidationMsg); err != nil {
						log.Printf("Failed to unmarshal invalidation message: %v", err)
						continue
					}

					// Send the account IDs to the channel
					ch <- invalidationMsg.AccountIDs
				}
			}
		}
	}()

	return ch
}

func main() {
	// Setup Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create cache service
	cacheService := NewCacheService(rdb)

	// Start listening for invalidation events in background
	invalidationChan := cacheService.SubscribeToInvalidationEvents()

	// Process invalidation events (in a real app, this would update some state)
	go func() {
		for accountIDs := range invalidationChan {
			log.Printf("Received invalidation event for accounts: %v", accountIDs)
			// In a real implementation, we might want to prefetch fresh data from DB
			// or perform other cache warming operations
		}
	}()

	// Keep the service running
	log.Println("Cache service started")
	select {} // Block forever
}