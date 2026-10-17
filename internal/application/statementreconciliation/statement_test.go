package statementreconciliation

import (
	"bytes"
	"strings"
	"testing"
)

const sampleHeader = "row_id,source_account_ref,period_start,period_end,posted_at,external_reference,direction,amount_minor,currency,opening_minor,closing_minor\n"
const sampleRow = "r1,synthetic-clearing,2026-10-01T00:00:00Z,2026-11-01T00:00:00Z,2026-10-15T12:00:00Z,transfer-1,debit,100,INR,1000,900\n"

func TestParseExactStatement(t *testing.T) {
	raw := []byte(sampleHeader + sampleRow)
	got, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].AmountMinor != 100 || got.OpeningMinor != 1000 || got.ClosingMinor != 900 || got.SourceAccountRef != "synthetic-clearing" {
		t.Fatalf("unexpected statement: %+v", got)
	}
	raw[0] = 'X'
	if !bytes.Equal(got.Source, []byte(sampleHeader+sampleRow)) {
		t.Fatal("source bytes alias caller memory")
	}
}

func TestStatementRejectsAmbiguousOrIncompleteInput(t *testing.T) {
	cases := map[string]string{
		"empty": "", "BOM": "\ufeff" + sampleHeader + sampleRow,
		"unknown header":   strings.Replace(sampleHeader, "row_id", "unknown", 1) + sampleRow,
		"duplicate header": strings.Replace(sampleHeader, "external_reference", "row_id", 1) + sampleRow,
		"reordered header": strings.Replace(sampleHeader, "row_id,source_account_ref", "source_account_ref,row_id", 1) + sampleRow,
		"no rows":          sampleHeader, "truncated": sampleHeader + strings.TrimSuffix(sampleRow, ",900\n"),
		"negative amount": sampleHeader + strings.Replace(sampleRow, ",100,INR", ",-100,INR", 1),
		"zero amount":     sampleHeader + strings.Replace(sampleRow, ",100,INR", ",0,INR", 1),
		"leading zero":    sampleHeader + strings.Replace(sampleRow, ",100,INR", ",0100,INR", 1),
		"fraction":        sampleHeader + strings.Replace(sampleRow, ",100,INR", ",1.00,INR", 1),
		"overflow":        sampleHeader + strings.Replace(sampleRow, ",100,INR", ",9223372036854775808,INR", 1),
		"wrong currency":  sampleHeader + strings.Replace(sampleRow, ",INR,", ",USD,", 1),
		"year zero":       sampleHeader + strings.Replace(sampleRow, "2026-10-01T00:00:00Z", "0000-10-01T00:00:00Z", 1),
		"offset timezone": sampleHeader + strings.ReplaceAll(sampleRow, "T00:00:00Z", "T01:00:00+01:00"),
		"before period":   sampleHeader + strings.Replace(sampleRow, "2026-10-15T12:00:00Z", "2026-09-30T23:59:59Z", 1),
		"end excluded":    sampleHeader + strings.Replace(sampleRow, "2026-10-15T12:00:00Z", "2026-11-01T00:00:00Z", 1),
		"bad totals":      sampleHeader + strings.Replace(sampleRow, ",1000,900", ",1000,899", 1),
		"duplicate row":   sampleHeader + sampleRow + sampleRow,
		"NUL":             sampleHeader + strings.Replace(sampleRow, "r1,", "r\x001,", 1),
		"invalid UTF8":    sampleHeader + string([]byte{0xff}) + sampleRow,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(raw)); err == nil {
				t.Fatal("invalid statement accepted")
			}
		})
	}
}

func TestRepeatedAmountsRemainSeparateAndMetadataMustAgree(t *testing.T) {
	first := strings.Replace(sampleRow, ",1000,900", ",1000,800", 1)
	second := strings.Replace(first, "r1,", "r2,", 1)
	if got, err := Parse([]byte(sampleHeader + first + second)); err != nil || len(got.Rows) != 2 {
		t.Fatalf("valid repeated amount lost: %v", err)
	}
	for _, second := range []string{strings.Replace(second, "synthetic-clearing", "other", 1), strings.Replace(second, ",1000,800", ",1000,700", 1)} {
		if _, err := Parse([]byte(sampleHeader + first + second)); err == nil {
			t.Fatal("mixed metadata accepted")
		}
	}
}

func TestStatementUsesWideTotalsAndBoundsSource(t *testing.T) {
	debit := strings.Replace(sampleRow, ",100,INR,1000,900", ",9223372036854775807,INR,0,0", 1)
	credit := strings.ReplaceAll(strings.Replace(debit, "r1,", "r2,", 1), ",debit,", ",credit,")
	if _, err := Parse([]byte(sampleHeader + debit + credit)); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(bytes.Repeat([]byte{'x'}, MaxSourceBytes+1)); err == nil {
		t.Fatal("oversized source accepted")
	}
}

func TestMatchNeverSelectsAnArbitraryCandidate(t *testing.T) {
	rows := []Row{{ID: "r1", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "INR"}}
	items := []LedgerItem{{ID: "p1", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "INR"}, {ID: "p2", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "INR"}}
	got := MatchExact(rows, items)
	if len(got.Matches) != 0 || got.Rows[0].Status != "ambiguous" {
		t.Fatalf("arbitrary match: %+v", got)
	}
	rows = append(rows, Row{ID: "r2", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "INR"})
	got = MatchExact(rows, items[:1])
	if len(got.Matches) != 0 || got.Rows[0].Status != "ambiguous" || got.Rows[1].Status != "ambiguous" {
		t.Fatalf("shared posting consumed: %+v", got)
	}
}

func TestMatchExactMoneyAndDirection(t *testing.T) {
	rows := []Row{{ID: "r1", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "INR"}}
	for _, item := range []LedgerItem{{ID: "p", Reference: "ref", Direction: "credit", AmountMinor: 100, Currency: "INR"}, {ID: "p", Reference: "ref", Direction: "debit", AmountMinor: 99, Currency: "INR"}, {ID: "p", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "USD"}} {
		if got := MatchExact(rows, []LedgerItem{item}); len(got.Matches) != 0 {
			t.Fatal("unequal item matched")
		}
	}
	item := LedgerItem{ID: "p", Reference: "ref", Direction: "debit", AmountMinor: 100, Currency: "INR"}
	if got := MatchExact(rows, []LedgerItem{item}); len(got.Matches) != 1 || got.Matches[0].PostingID != "p" {
		t.Fatalf("exact item not matched: %+v", got)
	}
}
