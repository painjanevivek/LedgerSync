package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
)

// AuthService handles JWT tokens for read-your-writes consistency
type AuthService struct {
	rdb       *redis.Client
	jwtSecret []byte
	ctx       context.Context
}

// NewAuthService creates a new AuthService instance
func NewAuthService(rdb *redis.Client, jwtSecret string) *AuthService {
	return &AuthService{
		rdb:       rdb,
		jwtSecret: []byte(jwtSecret),
		ctx:       context.Background(),
	}
}

// GenerateRYEWToken creates a read-your-writes token (JWT) for consistent reads
func (s *AuthService) GenerateRYEWToken(accountIDs []string) (string, error) {
	// Create token with account IDs and expiration
	claims := jwt.MapClaims{
		"account_ids": accountIDs,
		"issued_at":   time.Now().Unix(),
		"expires_at":  time.Now().Add(time.Hour * 24).Unix(), // 24 hour expiration
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Store token in Redis for validation (optional - could be stateless)
	err = s.rdb.Set(s.ctx, "ryew_token:"+tokenString, "valid", 0).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store token in Redis: %w", err)
	}

	return tokenString, nil
}

// ValidateRYEWToken validates a read-your-writes token and returns associated account IDs
func (s *AuthService) ValidateRYEWToken(tokenString string) ([]string, error) {
	// Check if token exists in Redis (for blacklisting capability)
	val, err := s.rdb.Get(s.ctx, "ryew_token:"+tokenString).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("token not found or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check token in Redis: %w", err)
	}

	// Parse and validate JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Extract account IDs from token
		if accountIDsInterface, ok := claims["account_ids"].([]interface{}); ok {
			accountIDs := make([]string, len(accountIDsInterface))
			for i, id := range accountIDsInterface {
				if str, ok := id.(string); ok {
					accountIDs[i] = str
				} else {
					return nil, fmt.Errorf("invalid account ID type in token")
				}
			}
			return accountIDs, nil
		}
		return nil, fmt.Errorf("invalid token claims")
	}

	return nil, fmt.Errorf("invalid token")
}

// InvalidateRYEWToken invalidates a token (for logout/security purposes)
func (s *AuthService) InvalidateRYEWToken(tokenString string) error {
	err := s.rdb.Del(s.ctx, "ryew_token:"+tokenString).Err()
	if err != nil {
		return fmt.Errorf("failed to invalidate token: %w", err)
	}
	return nil
}

// GetAccountIDsFromToken extracts account IDs from a valid token without full validation
func (s *AuthService) GetAccountIDsFromToken(tokenString string) ([]string, error) {
	// For performance, we can skip Redis lookup if we trust the token signature
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if accountIDsInterface, ok := claims["account_ids"].([]interface{}); ok {
			accountIDs := make([]string, len(accountIDsInterface))
			for i, id := range accountIDsInterface {
				if str, ok := id.(string); ok {
					accountIDs[i] = str
				} else {
					return nil, fmt.Errorf("invalid account ID type in token")
				}
			}
			return accountIDs, nil
		}
	}

	return nil, fmt.Errorf("invalid token claims")
}

func main() {
	# Setup Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		Password: "", # no password set
		DB:       0,  # use default DB
	})

	# Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	# JWT secret (in production, this should come from secure config/env)
	jwtSecret := "your-256-bit-secret-key-must-be-32-bytes-long!!!"

	# Create auth service
	authService := NewAuthService(rdb, jwtSecret)

	# For demonstration, generate a sample token
	sampleAccountIDs := []string{"acc_001", "acc_002"}
	token, err := authService.GenerateRYEWToken(sampleAccountIDs)
	if err != nil {
		log.Fatalf("Failed to generate sample token: %v", err)
	}
	log.Printf("Generated sample RYEW token: %s", token)

	# Validate the token
	accountIDs, err := authService.ValidateRYEWToken(token)
	if err != nil {
		log.Fatalf("Failed to validate sample token: %v", err)
	}
	log.Printf("Validated token for accounts: %v", accountIDs)

	# Keep service running (in reality, this would expose gRPC/HTTP endpoints)
	log.Println("Auth service started")
	select {} # Block forever
}