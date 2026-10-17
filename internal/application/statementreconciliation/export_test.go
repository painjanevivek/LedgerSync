package statementreconciliation

import "testing"

func TestSpreadsheetTextNeverDispatchesAFormula(t *testing.T) {
	for _, s := range []string{"=1+1", "+SUM(A1:A2)", "-CMD()", "@SUM(A1)", " \t=HYPERLINK()", "\r\n+value"} {
		if got := SpreadsheetText(s); got != "'"+s {
			t.Fatalf("unsafe spreadsheet text: %q", got)
		}
	}
	for _, s := range []string{"reference-1", "ordinary comment", ""} {
		if got := SpreadsheetText(s); got != s {
			t.Fatal("ordinary text changed")
		}
	}
}

func TestExportRejectsPreparedOrMissingEvidence(t *testing.T) {
	if _, err := ExportSnapshot(RunSnapshot{Header: RunHeader{State: "prepared"}}); err == nil {
		t.Fatal("prepared run exported as empty evidence")
	}
	if _, err := ExportSnapshot(RunSnapshot{Header: RunHeader{State: "completed"}, Rows: []Row{{ID: "r1"}}}); err == nil {
		t.Fatal("missing review evidence exported")
	}
}
