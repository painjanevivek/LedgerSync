// Package startup provides bounded dependency connection retries for process
// initialization. Runtime financial operations must handle dependency failures
// explicitly and do not use this package to retry ambiguous writes.
package startup

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

type Config struct {
	Timeout        time.Duration
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	OnRetry        func(Event)
	Jitter         func(time.Duration) time.Duration
	Sleep          func(context.Context, time.Duration) error
}

type Event struct {
	Dependency string
	Attempt    int
	Category   string
	Delay      time.Duration
	Remaining  time.Duration
	Err        error
}

func Open[T any](parent context.Context, dependency string, config Config, operation func(context.Context) (T, error)) (T, error) {
	var zero T
	dependency = strings.TrimSpace(dependency)
	if parent == nil {
		return zero, errors.New("startup context is required")
	}
	if dependency == "" || operation == nil {
		return zero, errors.New("startup dependency and operation are required")
	}
	config = withDefaults(config)
	if config.InitialBackoff > config.MaxBackoff || config.MaxBackoff > config.Timeout {
		return zero, errors.New("startup retry durations must satisfy initial backoff <= max backoff <= timeout")
	}
	ctx, cancel := context.WithTimeout(parent, config.Timeout)
	defer cancel()

	backoff := config.InitialBackoff
	for attempt := 1; ; attempt++ {
		resource, err := operation(ctx)
		if err == nil {
			return resource, nil
		}
		transient, category := classify(err)
		if !transient {
			return zero, fmt.Errorf("%s startup failed permanently on attempt %d: %w", dependency, attempt, err)
		}
		delay := config.Jitter(backoff)
		if delay <= 0 {
			delay = backoff
		}
		if config.OnRetry != nil {
			deadline, _ := ctx.Deadline()
			remaining := time.Until(deadline)
			if remaining < 0 {
				remaining = 0
			}
			config.OnRetry(Event{Dependency: dependency, Attempt: attempt, Category: category, Delay: delay, Remaining: remaining, Err: err})
		}
		if sleepErr := config.Sleep(ctx, delay); sleepErr != nil {
			return zero, fmt.Errorf("%s startup exhausted after %d attempts: %w (last dependency error: %v)", dependency, attempt, sleepErr, err)
		}
		if backoff < config.MaxBackoff {
			backoff *= 2
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		}
	}
}

func IsTransient(err error) bool {
	transient, _ := classify(err)
	return transient
}

func classify(err error) (bool, string) {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, "context"
	}
	message := strings.ToLower(err.Error())
	for _, permanent := range []string{
		"password authentication failed",
		"invalid password",
		"wrongpass",
		"noauth",
		"database driver and dsn are required",
		"cannot parse",
		"invalid dsn",
		"unknown driver",
		"tls configuration",
	} {
		if strings.Contains(message, permanent) {
			return false, "configuration"
		}
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true, "network"
	}
	for category, patterns := range map[string][]string{
		"network": {
			"connection refused",
			"connection reset",
			"network is unreachable",
			"no such host",
			"server misbehaving",
			"temporary failure in name resolution",
			"hostname resolving error",
			"i/o timeout",
			"cannot assign requested address",
		},
		"transport": {
			"broken pipe",
			"unexpected eof",
		},
		"dependency_loading": {
			"redis is loading",
			"loading redis",
		},
		"dependency_starting": {
			"database system is starting up",
		},
	} {
		for _, pattern := range patterns {
			if strings.Contains(message, pattern) {
				return true, category
			}
		}
	}
	return false, "unknown"
}

func withDefaults(config Config) Config {
	if config.Timeout <= 0 {
		config.Timeout = 90 * time.Second
	}
	if config.InitialBackoff <= 0 {
		config.InitialBackoff = 250 * time.Millisecond
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = 5 * time.Second
	}
	if config.Jitter == nil {
		config.Jitter = jitter
	}
	if config.Sleep == nil {
		config.Sleep = sleepWithContext
	}
	return config
}

func jitter(delay time.Duration) time.Duration {
	spread := delay / 5
	if spread <= 0 {
		return delay
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(spread*2+1)))
	if err != nil {
		return delay
	}
	return delay - spread + time.Duration(value.Int64())
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
