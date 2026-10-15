package operations

import "time"

type EventOrder struct {
	NodeID           string `json:"node_id"`
	LogicalTimestamp string `json:"logical_timestamp"`
	ParentReference  string `json:"parent_reference,omitempty"`
}
type PublicationEvidence struct {
	Generation     string     `json:"generation"`
	WorkerID       string     `json:"worker_id"`
	State          string     `json:"state"`
	StartedAt      time.Time  `json:"started_at"`
	LeaseUntil     time.Time  `json:"lease_until"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	ReceiveCounter string     `json:"receive_counter,omitempty"`
	SendCounter    string     `json:"send_counter,omitempty"`
}
type ReceiptEvidence struct {
	NodeID         string    `json:"node_id"`
	SenderNodeID   string    `json:"sender_node_id,omitempty"`
	SendCounter    string    `json:"send_counter,omitempty"`
	ReceiveCounter string    `json:"receive_counter"`
	ReceivedAt     time.Time `json:"received_at"`
}
