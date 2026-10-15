package httptransport

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type committedResponseObserverCapture struct {
	metadataUnavailable []string
	writeFailures       []string
}

func (capture *committedResponseObserverCapture) ObserveCommittedResponseMetadataUnavailable(_ context.Context, commandKind string) {
	capture.metadataUnavailable = append(capture.metadataUnavailable, commandKind)
}

func (capture *committedResponseObserverCapture) ObserveCommittedResponseWriteFailure(_ context.Context, commandKind string) {
	capture.writeFailures = append(capture.writeFailures, commandKind)
}

type failingCommittedWriter struct {
	header http.Header
	status int
}

func (writer *failingCommittedWriter) Header() http.Header {
	if writer.header == nil {
		writer.header = make(http.Header)
	}
	return writer.header
}

func (writer *failingCommittedWriter) WriteHeader(status int) { writer.status = status }

func (*failingCommittedWriter) Write([]byte) (int, error) {
	return 0, errors.New("client disconnected")
}

func TestWriteCommittedJSONNeverRewritesStatusAfterBodyFailure(t *testing.T) {
	writer := &failingCommittedWriter{}
	observer := &committedResponseObserverCapture{}
	WriteCommittedJSON(context.Background(), writer, CommittedResponse{
		Status:       http.StatusCreated,
		CommandKind:  "transfer",
		CommandID:    "transfer-1",
		RecoveryPath: "/api/transfers/transfer-1",
		Body:         map[string]string{"transfer_id": "transfer-1", "status": "posted"},
	}, observer)

	if writer.status != http.StatusCreated {
		t.Fatalf("status=%d want=%d", writer.status, http.StatusCreated)
	}
	if len(observer.writeFailures) != 1 || observer.writeFailures[0] != "transfer" {
		t.Fatalf("write failures=%v", observer.writeFailures)
	}
}

func TestWriteCommittedJSONFallsBackToRecoveryEnvelopeWhenEncodingFails(t *testing.T) {
	writer := &responseBuffer{header: make(http.Header)}
	observer := &committedResponseObserverCapture{}
	WriteCommittedJSON(context.Background(), writer, CommittedResponse{
		Status:       http.StatusCreated,
		CommandKind:  "funding",
		CommandID:    "funding-1",
		RecoveryPath: "/api/funding-events/funding-1",
		Body:         make(chan int),
	}, observer)

	if writer.status != http.StatusCreated {
		t.Fatalf("status=%d body=%s", writer.status, writer.body.String())
	}
	body := writer.body.String()
	for _, expected := range []string{`"outcome":"committed"`, `"metadata_status":"unavailable"`, `"command_id":"funding-1"`, `"recovery_path":"/api/funding-events/funding-1"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("fallback body=%s missing %s", body, expected)
		}
	}
	if len(observer.metadataUnavailable) != 1 || observer.metadataUnavailable[0] != "funding" {
		t.Fatalf("metadata observations=%v", observer.metadataUnavailable)
	}
}

type responseBuffer struct {
	header http.Header
	status int
	body   strings.Builder
}

func (writer *responseBuffer) Header() http.Header             { return writer.header }
func (writer *responseBuffer) WriteHeader(status int)          { writer.status = status }
func (writer *responseBuffer) Write(value []byte) (int, error) { return writer.body.Write(value) }
