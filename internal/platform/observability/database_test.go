package observability

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestDatabasePoolSignalsAreCumulativeBoundedAndUnregistered(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	telemetry := &Telemetry{enabled: true, meter: provider.Meter("test")}
	stop, err := telemetry.RegisterDatabasePool("api", func() sql.DBStats {
		return sql.DBStats{MaxOpenConnections: 4, OpenConnections: 3, InUse: 2, Idle: 1, WaitCount: 7, WaitDuration: 250 * time.Millisecond}
	})
	if err != nil {
		t.Fatal(err)
	}
	var result metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &result); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, scope := range result.ScopeMetrics {
		for _, metric := range scope.Metrics {
			switch data := metric.Data.(type) {
			case metricdata.Gauge[int64]:
				for _, point := range data.DataPoints {
					if point.Attributes.Len() != 2 {
						t.Fatal("pool metrics must have only role and state labels")
					}
					state, _ := point.Attributes.Value("state")
					counts[state.AsString()] = point.Value
				}
			case metricdata.Sum[int64]:
				if !data.IsMonotonic || len(data.DataPoints) != 1 || data.DataPoints[0].Value != 7 {
					t.Fatal("pool wait count is not a cumulative monotonic counter")
				}
			case metricdata.Sum[float64]:
				if !data.IsMonotonic || len(data.DataPoints) != 1 || data.DataPoints[0].Value != 0.25 {
					t.Fatal("pool wait duration must use cumulative seconds")
				}
			}
		}
	}
	if counts["in_use"] != 2 || counts["idle"] != 1 || counts["open"] != 3 || counts["limit"] != 4 {
		t.Fatalf("pool state missing: %v", counts)
	}
	stop()
	stop()
	result = metricdata.ResourceMetrics{}
	if err := reader.Collect(context.Background(), &result); err != nil {
		t.Fatal(err)
	}
	for _, scope := range result.ScopeMetrics {
		if len(scope.Metrics) != 0 {
			t.Fatal("closed pool remains registered")
		}
	}
	for _, name := range []string{"tenant-a", "", "api?token=private"} {
		if _, err := telemetry.RegisterDatabasePool(name, func() sql.DBStats { return sql.DBStats{} }); err == nil {
			t.Fatal("unbounded pool label accepted")
		}
	}
}

func TestDatabaseLockSignalHasNoKeyOrRawError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	telemetry := &Telemetry{enabled: true, meter: provider.Meter("test")}
	if err := telemetry.initializeDatabaseMetrics(); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), databaseObserverKey{}, telemetry)
	ObserveDatabaseLock(ctx, time.Now().Add(-time.Millisecond), context.DeadlineExceeded)
	var result metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &result); err != nil {
		t.Fatal(err)
	}
	for _, scope := range result.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "ledgersync.database.policy_lock.duration" {
				continue
			}
			data := metric.Data.(metricdata.Histogram[float64])
			if len(data.DataPoints) != 1 || data.DataPoints[0].Count != 1 || data.DataPoints[0].Sum <= 0 || data.DataPoints[0].Attributes.Len() != 1 {
				t.Fatal("invalid or private lock observation")
			}
			outcome, _ := data.DataPoints[0].Attributes.Value("outcome")
			if outcome.AsString() != "error" {
				t.Fatal("failed lock wait was not recorded")
			}
			return
		}
	}
	t.Fatal("policy lock metric missing")
}
