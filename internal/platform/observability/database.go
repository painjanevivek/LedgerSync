package observability

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type databaseObserverKey struct{}

func (t *Telemetry) initializeDatabaseMetrics() error {
	var err error
	t.databaseLockDuration, err = t.meter.Float64Histogram("ledgersync.database.policy_lock.duration", metric.WithUnit("ms"))
	return err
}

// RegisterDatabasePool reads in-process counters only. It makes no SQL call,
// opens no connection and exports no DSN, query, tenant or account identifier.
// Unregister before closing the pool. The returned stop function is idempotent.
func (t *Telemetry) RegisterDatabasePool(role string, stats func() sql.DBStats) (func(), error) {
	if (role != "api" && role != "worker" && role != "cron_worker") || stats == nil {
		return nil, errors.New("fixed database pool role and stats callback required")
	}
	if t == nil || !t.enabled {
		return func() {}, nil
	}
	connections, err := t.meter.Int64ObservableGauge("ledgersync.database.pool.connections")
	if err != nil {
		return nil, err
	}
	waits, err := t.meter.Int64ObservableCounter("ledgersync.database.pool.waits")
	if err != nil {
		return nil, err
	}
	duration, err := t.meter.Float64ObservableCounter("ledgersync.database.pool.wait.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	registration, err := t.meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		value := stats()
		for state, count := range map[string]int{"open": value.OpenConnections, "in_use": value.InUse, "idle": value.Idle, "limit": value.MaxOpenConnections} {
			observer.ObserveInt64(connections, int64(count), metric.WithAttributes(attribute.String("pool", role), attribute.String("state", state)))
		}
		labels := metric.WithAttributes(attribute.String("pool", role))
		observer.ObserveInt64(waits, value.WaitCount, labels)
		observer.ObserveFloat64(duration, value.WaitDuration.Seconds(), labels)
		return nil
	}, connections, waits, duration)
	if err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { _ = registration.Unregister() }) }, nil
}

// ObserveDatabaseLock measures the existing tenant-policy advisory-lock call,
// including one database round trip. It is not a measurement of all row locks.
// The context contains telemetry only, never a lock key or database authority.
func ObserveDatabaseLock(ctx context.Context, started time.Time, err error) {
	t, _ := ctx.Value(databaseObserverKey{}).(*Telemetry)
	if t == nil || !t.enabled || t.databaseLockDuration == nil {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	t.databaseLockDuration.Record(ctx, max(0, float64(time.Since(started))/float64(time.Millisecond)), metric.WithAttributes(attribute.String("outcome", outcome)))
}
