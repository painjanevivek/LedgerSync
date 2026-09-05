// Package redisconn centralizes Redis client configuration for local and
// managed deployments.
package redisconn

import (
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Options accepts the host:port form used by Docker Compose as well as
// provider-issued redis:// and rediss:// URLs with credentials and TLS.
func Options(address string) (*redis.Options, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("redis address is required")
	}
	if strings.Contains(address, "://") {
		options, err := redis.ParseURL(address)
		if err != nil {
			return nil, err
		}
		return options, nil
	}
	return &redis.Options{Addr: address}, nil
}
