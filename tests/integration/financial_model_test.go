package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
)

func (s *financialModelSuite) run(seed int64, operations []oracle.Operation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := s.reset(ctx); err != nil {
		return err
	}
	for index, operation := range operations {
		err := s.step(ctx, seed, index, operation)
		if err == nil {
			err = s.check(ctx)
		}
		if err != nil {
			var mismatch *modelMismatch
			if errors.As(err, &mismatch) {
				mismatch.Step = index
			}
			return fmt.Errorf("seed=%d step=%d kind=%s: %w", seed, index, operation.Kind, err)
		}
	}
	return nil
}

func TestGeneratedFinancialModel(t *testing.T) {
	type runCase struct {
		Seed       int64              `json:"seed"`
		Operations []oracle.Operation `json:"operations"`
	}
	cases := []runCase{}
	for _, seed := range []int64{17, 29, 101, 404, 20261015} {
		cases = append(cases, runCase{seed, oracle.Generate(seed, 64)})
	}
	if raw := os.Getenv("LEDGERSYNC_MODEL_REPLAY_JSON"); raw != "" {
		var replay runCase
		if len(raw) > 24000 || json.Unmarshal([]byte(raw), &replay) != nil || len(replay.Operations) < 1 || len(replay.Operations) > 256 {
			t.Fatal("invalid bounded model replay")
		}
		for _, op := range replay.Operations {
			valid := false
			for _, kind := range []string{"transfer", "replay", "conflict", "fund", "correct", "lost-response", "rollback", "concurrent", "insufficient"} {
				valid = valid || op.Kind == kind
			}
			if !valid || op.Minor < 0 || op.Minor > 1_000_000 || op.Reference < 0 || op.Reference > 255 {
				t.Fatal("invalid bounded model operation")
			}
		}
		cases = []runCase{replay}
	}
	for _, selected := range cases {
		seed, operations := selected.Seed, selected.Operations
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			suite := newFinancialModelSuite(t)
			err := suite.run(seed, operations)
			report := map[string]any{"seed": seed, "operations": operations, "passed": err == nil, "steps": len(operations), "scope": "restricted API role; isolated PostgreSQL; synthetic data"}
			if err != nil {
				report["error"] = err.Error()
				var observed *modelMismatch
				if errors.As(err, &observed) {
					prefix := operations[:observed.Step+1]
					class := observed.Class
					report["mismatch_class"] = class
					report["minimal_operations"] = oracle.Minimize(prefix, 20, func(candidate []oracle.Operation) bool {
						var failure *modelMismatch
						return errors.As(suite.run(seed, candidate), &failure) && failure.Class == class
					})
				}
			} else {
				report["journals"] = suite.ledger.Journals
				report["final_accounts"] = suite.ledger.Accounts
			}
			encoded, _ := json.Marshal(report)
			t.Logf("MODEL_REPORT %s", encoded)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
