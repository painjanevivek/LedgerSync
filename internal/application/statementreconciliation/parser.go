// Package statementreconciliation compares supplied statement evidence with a
// frozen ledger snapshot. It never posts or adjusts money.
package statementreconciliation

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"
)

const MaxSourceBytes = 1 << 20
const MaxRows = 1000
const Header = "row_id,source_account_ref,period_start,period_end,posted_at,external_reference,direction,amount_minor,currency,opening_minor,closing_minor"

var token = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
var unsigned = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
var signed = regexp.MustCompile(`^(0|-?[1-9][0-9]{0,18})$`)
var ErrInvalidStatement = errors.New("invalid statement")

type ValidationError struct {
	Row  int
	Code string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("statement row %d: %s", e.Row, e.Code) }
func (e *ValidationError) Unwrap() error { return ErrInvalidStatement }

type Row struct {
	ID          string    `json:"row_id"`
	Reference   string    `json:"external_reference"`
	Direction   string    `json:"direction"`
	AmountMinor int64     `json:"amount_minor,string"`
	Currency    string    `json:"currency"`
	PostedAt    time.Time `json:"posted_at"`
}
type Statement struct {
	Source           []byte    `json:"-"`
	SHA256           [32]byte  `json:"-"`
	SourceAccountRef string    `json:"source_account_ref"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	OpeningMinor     int64     `json:"opening_minor,string"`
	ClosingMinor     int64     `json:"closing_minor,string"`
	Rows             []Row     `json:"rows"`
}

func canonicalTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02T15:04:05Z", s)
	if err != nil || t.Year() < 1 || t.UTC().Format("2006-01-02T15:04:05Z") != s {
		return time.Time{}, ErrInvalidStatement
	}
	return t.UTC(), nil
}
func minor(s string, positive bool) (int64, error) {
	if (positive && !unsigned.MatchString(s)) || (!positive && !signed.MatchString(s)) {
		return 0, ErrInvalidStatement
	}
	return strconv.ParseInt(s, 10, 64)
}
func Parse(source []byte) (Statement, error) {
	fail := func(row int, code string) (Statement, error) { return Statement{}, &ValidationError{row, code} }
	if len(source) == 0 || len(source) > MaxSourceBytes || !utf8.Valid(source) || bytes.HasPrefix(source, []byte{0xef, 0xbb, 0xbf}) || bytes.ContainsRune(source, 0) || bytes.ContainsRune(source, '"') {
		return fail(0, "encoding_or_size")
	}
	lines := bytes.Split(bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n")), []byte("\n"))
	for i, line := range lines {
		if len(line) == 0 && i != len(lines)-1 {
			return fail(i+1, "blank_row")
		}
	}
	reader := csv.NewReader(bytes.NewReader(source))
	reader.FieldsPerRecord = 11
	header, err := reader.Read()
	if err != nil || !equalHeader(header) {
		return fail(1, "header")
	}
	result := Statement{Source: bytes.Clone(source), SHA256: sha256.Sum256(source), Rows: make([]Row, 0)}
	ids := map[string]bool{}
	total := new(big.Int)
	var metadata []string
	for n := 2; ; n++ {
		fields, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fail(n, "csv")
		}
		if len(result.Rows) >= MaxRows {
			return fail(n, "row_limit")
		}
		if !token.MatchString(fields[0]) || !token.MatchString(fields[1]) || !token.MatchString(fields[5]) || ids[fields[0]] {
			return fail(n, "row_identity")
		}
		if fields[8] != "INR" || (fields[6] != "debit" && fields[6] != "credit") {
			return fail(n, "currency_or_direction")
		}
		amount, err := minor(fields[7], true)
		if err != nil {
			return fail(n, "amount")
		}
		opening, err := minor(fields[9], false)
		if err != nil {
			return fail(n, "opening")
		}
		closing, err := minor(fields[10], false)
		if err != nil {
			return fail(n, "closing")
		}
		start, err := canonicalTime(fields[2])
		if err != nil {
			return fail(n, "period_start")
		}
		end, err := canonicalTime(fields[3])
		if err != nil || !end.After(start) {
			return fail(n, "period_end")
		}
		posted, err := canonicalTime(fields[4])
		if err != nil || posted.Before(start) || !posted.Before(end) {
			return fail(n, "posted_at")
		}
		current := []string{fields[1], fields[2], fields[3], fields[8], fields[9], fields[10]}
		if metadata == nil {
			metadata = current
			result.SourceAccountRef = fields[1]
			result.PeriodStart = start
			result.PeriodEnd = end
			result.OpeningMinor = opening
			result.ClosingMinor = closing
		} else {
			for i, value := range current {
				if metadata[i] != value {
					return fail(n, "mixed_metadata")
				}
			}
		}
		ids[fields[0]] = true
		result.Rows = append(result.Rows, Row{ID: fields[0], Reference: fields[5], Direction: fields[6], AmountMinor: amount, Currency: fields[8], PostedAt: posted})
		change := big.NewInt(amount)
		if fields[6] == "debit" {
			change.Neg(change)
		}
		total.Add(total, change)
	}
	if len(result.Rows) == 0 {
		return fail(2, "empty_statement")
	}
	expected := new(big.Int).Add(big.NewInt(result.OpeningMinor), total)
	if expected.Cmp(big.NewInt(result.ClosingMinor)) != 0 {
		return fail(0, "balance_total")
	}
	return result, nil
}
func equalHeader(fields []string) bool {
	expected := bytes.Split([]byte(Header), []byte{','})
	if len(fields) != len(expected) {
		return false
	}
	for i, s := range fields {
		if s != string(expected[i]) {
			return false
		}
	}
	return true
}
