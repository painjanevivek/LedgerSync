// Command reconcile is intentionally a separate operational entry point.
// Its database-backed reconciliation implementation is introduced with the
// operational phase; keeping the command now stabilizes the deployment API.
package main

import "log/slog"

func main() {
	slog.Info("reconciliation command is reserved for the operational implementation")
}
