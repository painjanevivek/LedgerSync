package integration_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"testing"
)

func TestBatchedLogicalReceiptsPreserveScopeOrderAndRollback(t *testing.T) {
	service, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	if _, err := service.Submit(ctx, transferCommand(t, "batch-receipt-fixture-key", "1.00")); err != nil {
		t.Fatal(err)
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(1)
	node := "batch-receiver-" + uuid.NewString()
	receiver, _ := db.NewLogicalClockRepository(worker.db, node)
	rows, err := admin.Query(`SELECT id::text FROM outbox_events ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	messages := []projection.Message{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, projection.Message{ID: fmt.Sprintf("100-%d", len(messages)), Event: outbox.Event{ID: id, TenantID: testTenantID, Delivery: &outbox.CausalMetadata{NodeID: "sender", Timestamp: 200}}})
	}
	rows.Close()
	if len(messages) != 3 {
		t.Fatal("three source events required")
	}
	broken := append([]projection.Message(nil), messages...)
	broken[1].Event.ID = uuid.NewString()
	if err := receiver.ReceiveBatch(ctx, broken); err == nil {
		t.Fatal("nonexistent event receipt accepted")
	}
	if countRows(t, admin, `SELECT count(*) FROM logical_message_receipts WHERE node_id=$1`, node) != 0 {
		t.Fatal("failed tenant batch retained a partial receipt")
	}
	if err := receiver.ReceiveBatch(ctx, messages); err != nil {
		t.Fatal(err)
	}
	var minimum, maximum int64
	if err := admin.QueryRow(`SELECT min(receive_counter),max(receive_counter) FROM logical_message_receipts WHERE node_id=$1`, node).Scan(&minimum, &maximum); err != nil || minimum != 201 || maximum != 203 {
		t.Fatal("batch changed Lamport receive order or failed rollback", err)
	}
	foreign := messages[0]
	foreign.Event.TenantID = "00000000-0000-4000-8000-000000009202"
	if err := receiver.ReceiveBatch(ctx, []projection.Message{foreign}); err == nil {
		t.Fatal("cross-tenant event accepted")
	}
	if err := receiver.ReceiveBatch(ctx, make([]projection.Message, 101)); err == nil {
		t.Fatal("unbounded receipt batch accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := receiver.ReceiveBatch(cancelled, messages); err == nil {
		t.Fatal("cancelled batch accepted")
	}
	if err := receiver.Receive(ctx, projection.Message{ID: "101-0", Event: messages[0].Event}); err != nil {
		t.Fatal("pool-size-one connection scope was not cleaned", err)
	}
}
