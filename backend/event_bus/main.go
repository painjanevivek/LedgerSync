package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
	pb "Distributed-system-FA1-FA2/backend/proto"
)

// EventBusService listens to Redis Streams events and handles them
type EventBusService struct {
	rdb   *redis.Client
	ctx   context.Context
	cache map[string]*AccountBalance // In-memory cache for demonstration
}

// AccountBalance represents an account balance in memory
type AccountBalance struct {
	AccountID string
	Balance   float64
	Currency  string
	UpdatedAt int64
}

// NewEventBusService creates a new EventBusService instance
func NewEventBusService(rdb *redis.Client) *EventBusService {
	return &EventBusService{
		rdb:   rdb,
		ctx:   context.Background(),
		cache: make(map[string]*AccountBalance),
	}
}

// StartListening begins listening for events on Redis Streams
func (s *EventBusService) StartListening() {
	go func() {
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
				// In a real system, you might want to implement retry logic here
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

					// Handle the invalidation event
					log.Printf("Received balance invalidation event for accounts: %v at %d", invalidationMsg.AccountIDs, invalidationMsg.Timestamp)
					s.handleInvalidationEvent(invalidationMsg.AccountIDs, invalidationMsg.Timestamp)
				}
			}
		}
	}()
}

// handleInvalidationEvent processes balance invalidation events
func (s *EventBusService) handleInvalidationEvent(accountIDs []string, timestamp int64) {
	// In a real implementation, this would:
	// 1. Fetch fresh data from the database for the invalidated accounts
	// 2. Update the cache with fresh data
	// 3. Notify any interested parties (like the auth service for RYEW token validation)

	// For demonstration, we'll just log the event and clear our in-memory cache
	for _, accountID := range accountIDs {
		delete(s.cache, accountID)
		log.Printf("Cleared cache for account %s", accountID)
	}

	// In a real system, you might publish a "cache warmed" event or update some metrics
}

// GetBalanceFromCache retrieves balance from our in-memory cache (for demonstration)
func (s *EventBusService) GetBalanceFromCache(accountID string) (*AccountBalance, bool) {
	balance, exists := s.cache[accountID]
	return balance, exists
}

// SetBalanceInCache stores balance in our in-memory cache (for demonstration)
func (s *EventBusService) SetBalanceInCache(accountID string, balance float64, currency string) {
	s.cache[accountID] = &AccountBalance{
		AccountID: accountID,
		Balance:   balance,
		Currency:  currency,
		UpdatedAt: timestampNow(),
	}
}

// timestampNow returns current timestamp in seconds
func timestampNow() int64 {
	return 0 // Placeholder - in real implementation use time.Now().Unix()
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

	// Create event bus service
	eventBus := NewEventBusService(rdb)

	// Start listening for events
	eventBus.StartListening()

	log.Println("Event bus service started and listening for balance invalidation events")

	// Keep the service running
	select {} // Block forever
}