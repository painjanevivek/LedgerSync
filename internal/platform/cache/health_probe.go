package cache

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type HealthProbe struct{ client *redis.Client }

func NewHealthProbe(client *redis.Client) (*HealthProbe, error) {
	if client == nil {
		return nil, errors.New("redis health client is required")
	}
	return &HealthProbe{client: client}, nil
}

func (p *HealthProbe) Ping(ctx context.Context) error {
	if p == nil || p.client == nil {
		return errors.New("redis health client is unavailable")
	}
	return p.client.Ping(ctx).Err()
}
