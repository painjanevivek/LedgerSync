package integration_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
)

func TestFinancialConcurrentHistoriesMatchSerialOrders(t *testing.T) {
	for index, commands := range [][]oracle.Operation{{{Kind: "transfer", Minor: 7000}, {Kind: "transfer", Minor: 6000}}, {{Kind: "transfer", Minor: 200}, {Kind: "transfer", Minor: 100, Reverse: true}, {Kind: "transfer", Minor: 300}, {Kind: "transfer", Minor: 200, Reverse: true}}} {
		suite := newFinancialModelSuite(t)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := suite.reset(ctx); err != nil {
			cancel()
			t.Fatal(err)
		}
		initial := oracle.New(map[string]oracle.Account{"a": {Minor: "10000"}, "b": {Minor: "2000"}})
		history := make([]oracle.CompletedOperation, len(commands))
		failures := make(chan error, len(commands))
		start := make(chan struct{})
		clock := time.Now()
		var group sync.WaitGroup
		for position, command := range commands {
			intent, request := suite.intent(int64(700+index), position, command)
			group.Add(1)
			go func(position int) {
				defer group.Done()
				<-start
				invoked := time.Since(clock).Nanoseconds() + 1
				result, err := suite.transfers.Submit(ctx, request)
				responded := time.Since(clock).Nanoseconds() + 1
				if err != nil {
					failures <- err
					return
				}
				version := result.Result.MinimumBalanceVersions[request.DebitAccountID.String()]
				history[position] = oracle.CompletedOperation{Intent: intent, Invoked: invoked, Responded: responded, Outcome: oracle.Outcome{Status: result.Result.Status, Replayed: result.Replayed, From: oracle.Account{Version: uint64(version)}}}
			}(position)
		}
		close(start)
		group.Wait()
		close(failures)
		for err := range failures {
			cancel()
			t.Fatal(err)
		}
		witness, err := oracle.CheckHistory(initial, history)
		encoded, _ := json.Marshal(map[string]any{"history": history, "serial_witness": witness, "passed": err == nil})
		t.Logf("CONCURRENT_MODEL_REPORT %s", encoded)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		for _, position := range witness {
			initial.Move(history[position].Intent)
		}
		suite.ledger = initial
		if err := suite.check(ctx); err != nil {
			cancel()
			t.Fatal(err)
		}
		cancel()
	}
}
