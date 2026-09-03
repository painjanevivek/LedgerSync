package httptransport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
)

type CommittedResponseObserver interface {
	ObserveCommittedResponseMetadataUnavailable(context.Context, string)
	ObserveCommittedResponseWriteFailure(context.Context, string)
}

// CommittedResponse freezes a durable command outcome before the HTTP write.
// Optional serialization failures fall back to a recovery envelope at the same
// success status. Once WriteHeader is called, no error response may be written.
type CommittedResponse struct {
	Status       int
	CommandKind  string
	CommandID    string
	RecoveryPath string
	Body         any
	Headers      http.Header
}

type committedRecoveryEnvelope struct {
	Outcome        string `json:"outcome"`
	MetadataStatus string `json:"metadata_status"`
	CommandID      string `json:"command_id"`
	RecoveryMethod string `json:"recovery_method"`
	RecoveryPath   string `json:"recovery_path"`
}

func WriteCommittedJSON(ctx context.Context, writer http.ResponseWriter, response CommittedResponse, observer CommittedResponseObserver) {
	for name, values := range response.Headers {
		for _, value := range values {
			writer.Header().Add(name, value)
		}
	}
	payload, err := json.Marshal(response.Body)
	if err != nil {
		observeCommittedMetadataUnavailable(ctx, observer, response.CommandKind)
		writer.Header().Set("X-LedgerSync-Metadata-Status", "unavailable")
		payload, _ = json.Marshal(committedRecoveryEnvelope{
			Outcome:        "committed",
			MetadataStatus: "unavailable",
			CommandID:      response.CommandID,
			RecoveryMethod: http.MethodGet,
			RecoveryPath:   response.RecoveryPath,
		})
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(response.Status)
	payload = append(payload, '\n')
	if _, err := writer.Write(payload); err != nil {
		if observer != nil {
			observer.ObserveCommittedResponseWriteFailure(ctx, response.CommandKind)
		}
		slog.ErrorContext(ctx, "committed response body write failed",
			"command_kind", response.CommandKind,
			"command_id_hash", hashCommittedCommandID(response.CommandID),
			"error", err,
		)
	}
}

func observeCommittedMetadataUnavailable(ctx context.Context, observer CommittedResponseObserver, commandKind string) {
	if observer != nil {
		observer.ObserveCommittedResponseMetadataUnavailable(ctx, commandKind)
	}
}

func hashCommittedCommandID(commandID string) string {
	if commandID == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(commandID))
	return hex.EncodeToString(digest[:8])
}
