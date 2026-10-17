package statementreconciliation

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strconv"
	"strings"
)

var reviewReason = regexp.MustCompile(`^[a-z][a-z_]{0,63}$`)
var reviewEvidence = regexp.MustCompile(`^/(transfers|funding|reconciliation)/[A-Za-z0-9/_-]+$`)

// SpreadsheetText neutralizes untrusted text without changing exact numeric
// columns. CSV quoting alone does not prevent spreadsheet formula execution.
func SpreadsheetText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		return "'" + value
	}
	return value
}
func ExportSnapshot(snapshot RunSnapshot) ([]byte, error) {
	if snapshot.Header.State != "completed" {
		return nil, ErrConflict
	}
	if len(snapshot.Reviews) < len(snapshot.Rows) {
		return nil, ErrUnavailable
	}
	if len(snapshot.Rows) > MaxRows || len(snapshot.Items) > MaxSnapshotItems || len(snapshot.Reviews) > MaxRows+MaxSnapshotItems {
		return nil, ErrLimit
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"record_type", "item_id", "external_reference", "direction", "amount_minor", "currency", "original_classification", "review_status", "assignee", "reason", "comment", "evidence", "review_version", "linked_posting_id", "run_id", "cutoff"}); err != nil {
		return nil, err
	}
	for _, item := range snapshot.Reviews {
		record := []string{item.Kind, item.ItemID, item.Reference, item.Direction, strconv.FormatInt(item.AmountMinor, 10), item.Currency, item.Classification, item.Status, item.Assignee, item.Reason, item.Comment, item.Evidence, strconv.FormatInt(item.Version, 10), item.LinkedPostingID, snapshot.Header.ID, snapshot.Header.Cutoff.UTC().Format("2006-01-02T15:04:05Z")}
		for i := range record {
			if i != 4 && i != 12 {
				record[i] = SpreadsheetText(record[i])
			}
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	if buffer.Len() > 4*MaxSourceBytes {
		return nil, ErrLimit
	}
	return buffer.Bytes(), nil
}
