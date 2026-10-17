package model

import "testing"

func workflowFixture() *Workflow {
	w := NewWorkflow()
	w.Grant("operator", "tenant", false)
	w.Grant("reviewer", "tenant", true)
	return w
}

func TestWorkflowPredictsLifecycleAuthorityAndOriginalReceipts(t *testing.T) {
	w := workflowFixture()
	create := WorkflowCommand{Kind: "create", Tenant: "tenant", Actor: "operator", Key: "create-a", Account: "a"}
	if got := w.Apply(create); got.Status != "created" || got.Version != 1 {
		t.Fatal(got)
	}
	freeze := WorkflowCommand{Kind: "freeze", Tenant: "tenant", Actor: "operator", Key: "freeze-a", Account: "a", ExpectedVersion: 1}
	if got := w.Apply(freeze); got.Status != "changed" || got.Version != 2 {
		t.Fatal(got)
	}
	if got := w.Apply(freeze); !got.Replayed || got.Version != 2 {
		t.Fatal("lifecycle replay lost original version")
	}
	changed := freeze
	changed.Kind = "reactivate"
	if got := w.Apply(changed); got.Status != "conflict" {
		t.Fatal("same lifecycle key accepted changed target")
	}
	changed.Key = "new-active"
	if got := w.Apply(changed); got.Status != "stale" {
		t.Fatal("stale lifecycle version accepted")
	}
	changed.ExpectedVersion = 2
	changed.Key = "new-active-correct-version"
	if got := w.Apply(changed); got.Status != "changed" || got.Version != 3 {
		t.Fatal(got)
	}
	closed := changed
	closed.Kind, closed.Key, closed.ExpectedVersion = "close", "close-a", 3
	if got := w.Apply(closed); got.Status != "changed" {
		t.Fatal(got)
	}
	changed.Key, changed.ExpectedVersion = "closed-active", 4
	if got := w.Apply(changed); got.Status != "terminal" {
		t.Fatal("closed account reactivated")
	}
	changed.Key, changed.ExpectedVersion = "closed-active-stale-version", 1
	if got := w.Apply(changed); got.Status != "terminal" {
		t.Fatal("closed account must stay terminal even with an old revision")
	}
	foreign := create
	foreign.Tenant, foreign.Account, foreign.Key = "other", "secret", "foreign"
	if got := w.Apply(foreign); got.Status != "forbidden" || len(w.Accounts) != 1 {
		t.Fatal("cross-tenant create changed state")
	}
	foreign.Tenant, foreign.Actor = "tenant", "unknown"
	if got := w.Apply(foreign); got.Status != "forbidden" {
		t.Fatal("missing actor authority accepted")
	}
}

func TestWorkflowPredictsIndependentApprovalAndStaleDecision(t *testing.T) {
	w := workflowFixture()
	w.Grant("requester", "tenant", true)
	w.Apply(WorkflowCommand{Kind: "create", Tenant: "tenant", Actor: "operator", Key: "create-a", Account: "a"})
	request := WorkflowCommand{Kind: "request-funding", Tenant: "tenant", Actor: "requester", Key: "request", Account: "a", Minor: "100"}
	if got := w.Apply(request); got.Status != "pending" {
		t.Fatal(got)
	}
	decision := WorkflowCommand{Kind: "approve", Tenant: "tenant", Actor: "requester", Key: "self-approve", Reference: "request"}
	if got := w.Apply(decision); got.Status != "forbidden" {
		t.Fatal("self approval accepted")
	}
	decision.Actor, decision.Key = "reviewer", "independent-approve"
	if got := w.Apply(decision); got.Status != "approved" {
		t.Fatal(got)
	}
	decision.Kind, decision.Key = "reject", "stale-reject"
	if got := w.Apply(decision); got.Status != "conflict" {
		t.Fatal("opposed stale decision accepted")
	}
	post := WorkflowCommand{Kind: "post-funding", Tenant: "tenant", Actor: "reviewer", Key: "post", Reference: "request"}
	if got := w.Apply(post); got.Status != "posted" || w.Ledger.Journals != 1 {
		t.Fatal(got)
	}
	if got := w.Apply(post); !got.Replayed || w.Ledger.Journals != 1 {
		t.Fatal("funding replay changed financial effect")
	}
	if err := w.Ledger.Check(); err != nil {
		t.Fatal(err)
	}
	close := WorkflowCommand{Kind: "close", Tenant: "tenant", Actor: "operator", Key: "close", Account: "a", ExpectedVersion: 1}
	if got := w.Apply(close); got.Status != "non_zero" {
		t.Fatal("funded account closed")
	}
}
