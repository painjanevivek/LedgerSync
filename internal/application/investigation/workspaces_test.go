package investigation

import "testing"

func TestWorkspaceCreateAcceptsOnlySafeAuthorizedContext(t *testing.T) {
	id := "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"
	command, err := NormalizeWorkspaceCreate(WorkspaceCreate{Title: "Delayed transfer review", Taxonomy: "transfer_delivery", QueryKind: "immutable_id", QueryValue: id, RootRecordType: "transfer", RootRecordID: id, Access: SearchAccess{Transfers: true}})
	if err != nil || command.QueryValue != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" || command.RootRecordID != command.QueryValue {
		t.Fatalf("command=%#v err=%v", command, err)
	}
	command, err = NormalizeWorkspaceCreate(WorkspaceCreate{Title: "Bank reference review", Taxonomy: "funding", QueryKind: "approved_reference", QueryValue: "BANK-REF-20260819", RootRecordType: "funding", RootRecordID: id, Access: SearchAccess{Funding: true}})
	if err != nil || command.QueryValue != "BANK-REF-20260819" {
		t.Fatalf("command=%#v err=%v", command, err)
	}

	invalid := []WorkspaceCreate{
		{Title: "Token=secret", Taxonomy: "other", QueryKind: "immutable_id", QueryValue: id, RootRecordType: "transfer", RootRecordID: id, Access: SearchAccess{Transfers: true}},
		{Title: "Customer@example.test", Taxonomy: "other", QueryKind: "immutable_id", QueryValue: id, RootRecordType: "transfer", RootRecordID: id, Access: SearchAccess{Transfers: true}},
		{Title: "Mismatch", Taxonomy: "other", QueryKind: "immutable_id", QueryValue: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", RootRecordType: "transfer", RootRecordID: id, Access: SearchAccess{Transfers: true}},
		{Title: "No scope", Taxonomy: "other", QueryKind: "immutable_id", QueryValue: id, RootRecordType: "transfer", RootRecordID: id, Access: SearchAccess{Accounts: true}},
		{Title: "Unknown taxonomy", Taxonomy: "incident", QueryKind: "immutable_id", QueryValue: id, RootRecordType: "transfer", RootRecordID: id, Access: SearchAccess{Transfers: true}},
	}
	for _, candidate := range invalid {
		if _, err := NormalizeWorkspaceCreate(candidate); err == nil {
			t.Fatalf("accepted %#v", candidate)
		}
	}
}

func TestWorkspaceIdentifiersVersionsAndPathsAreBounded(t *testing.T) {
	if subject, err := NormalizeWorkspaceSubject("operator-2"); err != nil || subject != "operator-2" {
		t.Fatalf("subject=%q err=%v", subject, err)
	}
	for _, subject := range []string{"", " operator", "operator\n2"} {
		if _, err := NormalizeWorkspaceSubject(subject); err == nil {
			t.Fatalf("accepted subject %q", subject)
		}
	}
	if version, err := ParseWorkspaceVersion("42"); err != nil || version != 42 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	for _, version := range []string{"", "0", "-1", "+1", "99999999999999999999"} {
		if _, err := ParseWorkspaceVersion(version); err == nil {
			t.Fatalf("accepted version %q", version)
		}
	}
	if path := WorkspaceTargetPath("reconciliation_mismatch", "id"); path != "" {
		t.Fatalf("unreleased mismatch path=%q", path)
	}
	if path := WorkspaceTargetPath("transfer", "id"); path != "/transfers/id" {
		t.Fatalf("transfer path=%q", path)
	}
}
