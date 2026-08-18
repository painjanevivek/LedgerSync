package db

import "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"

func firstTelemetry(values []*observability.Telemetry) *observability.Telemetry {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}
