package db

import (
	"strings"
	"testing"
)

func TestAuditMetadataPreservesBoundedUnicodeReasonAndRejectsUnsafeValues(t *testing.T) {
	reason := strings.Repeat("界", 256)
	clean := sanitizeAuditMetadata(map[string]string{
		"reason":       reason,
		"secret_token": "must-not-project",
	})
	if clean["reason"] != reason {
		t.Fatal("valid bounded Unicode lifecycle reason was not preserved")
	}
	if _, present := clean["secret_token"]; present {
		t.Fatal("secret-bearing audit metadata key was retained")
	}
	if got := sanitizeAuditMetadata(map[string]string{"reason": "line one\nline two"}); len(got) != 0 {
		t.Fatalf("control-bearing lifecycle reason was retained: %#v", got)
	}
	if got := sanitizeAuditMetadata(map[string]string{"reason": strings.Repeat("界", 257)}); len(got) != 0 {
		t.Fatalf("overlong lifecycle reason was retained: %#v", got)
	}
}
