package startup

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOpenRetriesTransientFailuresAndReturnsTheResource(t *testing.T) {
	attempts := 0
	events := make([]Event, 0, 2)
	resource, err := Open(context.Background(), "database", Config{
		Timeout:        time.Second,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
		OnRetry:        func(event Event) { events = append(events, event) },
		Jitter:         func(delay time.Duration) time.Duration { return delay },
		Sleep:          sleepWithContext,
	}, func(context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("connection refused")
		}
		return "connected", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if resource != "connected" || attempts != 3 {
		t.Fatalf("resource=%q attempts=%d", resource, attempts)
	}
	if len(events) != 2 || events[0].Category != "network" || events[0].Remaining <= 0 {
		t.Fatalf("retry events=%+v", events)
	}
}

func TestOpenFailsImmediatelyForPermanentConfigurationErrors(t *testing.T) {
	attempts := 0
	_, err := Open(context.Background(), "database", Config{
		Timeout:        time.Second,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
	}, func(context.Context) (string, error) {
		attempts++
		return "", errors.New("password authentication failed")
	})
	if err == nil || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestOpenStopsAtTheConfiguredDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	attempts := 0
	_, err := Open(ctx, "redis", Config{
		Timeout:        time.Second,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Jitter:         func(delay time.Duration) time.Duration { return delay },
		Sleep:          sleepWithContext,
	}, func(context.Context) (string, error) {
		attempts++
		return "", errors.New("LOADING Redis is loading the dataset in memory")
	})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || attempts < 2 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestIsTransientClassifiesKnownStartupFailures(t *testing.T) {
	for _, message := range []string{
		"connection refused",
		"lookup postgres on 127.0.0.11:53: server misbehaving",
		"LOADING Redis is loading the dataset in memory",
		"the database system is starting up",
		"i/o timeout",
	} {
		if !IsTransient(errors.New(message)) {
			t.Errorf("expected transient: %s", message)
		}
	}
	for _, message := range []string{
		"password authentication failed",
		"WRONGPASS invalid username-password pair",
		"database driver and DSN are required",
		"cannot parse database URL",
	} {
		if IsTransient(errors.New(message)) {
			t.Errorf("expected permanent: %s", message)
		}
	}
}
