package reservations

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const MaxExportHolds = 2000
const MaxExportBytes = 2 << 20

type Snapshot struct {
	AsOf  time.Time
	Holds []Hold
}

// Export represents obligations from one scoped database snapshot. A passed
// deadline never changes an active obligation into a release in this evidence.
func (s *Service) Export(ctx context.Context, scope Scope) ([]byte, error) {
	if !validScope(scope) {
		return nil, ErrInvalid
	}
	snapshot, err := s.repo.Snapshot(ctx, scope)
	if err != nil {
		return nil, err
	}
	if len(snapshot.Holds) > MaxExportHolds {
		return nil, ErrLimit
	}
	if snapshot.AsOf.IsZero() {
		return nil, ErrUnavailable
	}
	var buffer bytes.Buffer
	row := func(cells []string) {
		for n, cell := range cells {
			if n > 0 {
				buffer.WriteByte(',')
			}
			buffer.WriteByte('"')
			buffer.WriteString(strings.ReplaceAll(cell, `"`, `""`))
			buffer.WriteByte('"')
		}
		buffer.WriteByte('\n')
	}
	row([]string{"schema_version", "snapshot_as_of_utc", "hold_id", "state", "amount_minor", "reserved_minor", "currency", "debit_account_id", "credit_account_id", "created_by", "created_at_utc", "expires_at_utc", "completed_at_utc", "transfer_id", "hold_version"})
	for _, hold := range snapshot.Holds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		minor, e := strconv.ParseInt(hold.AmountMinor, 10, 64)
		if e != nil || minor <= 0 || strconv.FormatInt(minor, 10) != hold.AmountMinor {
			return nil, ErrUnavailable
		}
		reserved := "0"
		switch hold.State {
		case "active":
			reserved = hold.AmountMinor
		case "captured", "released", "expired":
		default:
			return nil, ErrUnavailable
		}
		completed := ""
		if hold.CompletedAt != nil {
			completed = hold.CompletedAt.UTC().Format(time.RFC3339Nano)
		}
		cells := []string{"1", snapshot.AsOf.UTC().Format(time.RFC3339Nano), hold.ID, hold.State, hold.AmountMinor, reserved, hold.Currency, hold.DebitAccountID, hold.CreditAccountID, hold.CreatedBy, hold.CreatedAt.UTC().Format(time.RFC3339Nano), hold.ExpiresAt.UTC().Format(time.RFC3339Nano), completed, hold.TransferID, hold.Version}
		for n, cell := range cells {
			trimmed := strings.TrimLeftFunc(cell, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf) })
			if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
				cells[n] = "'" + cell
			}
		}
		row(cells)
		if buffer.Len() > MaxExportBytes {
			return nil, ErrLimit
		}
	}
	return buffer.Bytes(), nil
}
