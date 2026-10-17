package model

import "math/big"

// Workflow is an independent bounded lifecycle and approval specification.
// Account revision and financial balance version are deliberately different.
type WorkflowAccount struct {
	Tenant, State, Owner string
	Version              uint64
}
type workflowActor struct {
	Tenant  string
	Finance bool
}
type workflowApproval struct {
	Tenant, Account, Requester, Minor, State string
}
type WorkflowCommand struct {
	Kind, Tenant, Actor, Key, Account, To, Minor, Reference string
	ExpectedVersion                                         uint64
}
type WorkflowResult struct {
	Status   string
	Version  uint64
	Replayed bool
}
type workflowReceipt struct {
	Command WorkflowCommand
	Result  WorkflowResult
}
type Workflow struct {
	Ledger   *Ledger
	Accounts map[string]WorkflowAccount
	actors   map[string]workflowActor
	requests map[string]workflowApproval
	receipts map[requestKey]workflowReceipt
}

func NewWorkflow() *Workflow {
	return &Workflow{Ledger: NewForCurrency(map[string]Account{}, "INR"), Accounts: map[string]WorkflowAccount{}, actors: map[string]workflowActor{}, requests: map[string]workflowApproval{}, receipts: map[requestKey]workflowReceipt{}}
}
func (w *Workflow) Grant(actor, tenant string, finance bool) {
	if actor == "" || tenant == "" || len(w.actors) >= 16 {
		panic("bounded explicit model authority required")
	}
	w.actors[actor] = workflowActor{Tenant: tenant, Finance: finance}
}

func workflowOperation(kind string) string {
	switch kind {
	case "freeze", "reactivate", "close":
		return "account-status"
	case "approve", "reject":
		return "funding-decision"
	default:
		return kind
	}
}

func (w *Workflow) Apply(c WorkflowCommand) WorkflowResult {
	actor, known := w.actors[c.Actor]
	if !known || actor.Tenant != c.Tenant {
		return WorkflowResult{Status: "forbidden"}
	}
	if c.Key == "" || len(c.Key) > 256 || len(w.receipts) >= 2048 {
		return WorkflowResult{Status: "invalid"}
	}
	key := requestKey{c.Tenant, c.Actor, workflowOperation(c.Kind), c.Key}
	if saved, ok := w.receipts[key]; ok {
		if saved.Command != c {
			return WorkflowResult{Status: "conflict"}
		}
		result := saved.Result
		result.Replayed = true
		return result
	}
	result := w.apply(c, actor)
	if result.Status != "forbidden" && result.Status != "not_found" && result.Status != "invalid" {
		w.receipts[key] = workflowReceipt{c, result}
	}
	return result
}

func (w *Workflow) apply(c WorkflowCommand, actor workflowActor) WorkflowResult {
	result := func(status string) WorkflowResult { return WorkflowResult{Status: status} }
	account, exists := w.Accounts[c.Account]
	switch c.Kind {
	case "create":
		if c.Account == "" || exists || len(w.Accounts) >= 64 {
			return result("conflict")
		}
		w.Accounts[c.Account] = WorkflowAccount{Tenant: c.Tenant, State: "active", Owner: c.Actor, Version: 1}
		w.Ledger.Accounts[c.Account] = Account{Minor: "0"}
		return WorkflowResult{Status: "created", Version: 1}
	case "freeze", "reactivate", "close":
		if !exists || account.Tenant != c.Tenant || account.Owner != c.Actor {
			return result("not_found")
		}
		if account.State == "closed" {
			return result("terminal")
		}
		if c.ExpectedVersion != account.Version {
			return result("stale")
		}
		target := map[string]string{"freeze": "frozen", "reactivate": "active", "close": "closed"}[c.Kind]
		if target == account.State {
			return result("invalid")
		}
		if target == "closed" {
			if w.Ledger.Accounts[c.Account].Minor != "0" {
				return result("non_zero")
			}
			for _, request := range w.requests {
				if request.Account == c.Account && (request.State == "pending" || request.State == "approved") {
					return result("obligations")
				}
			}
		}
		account.State, account.Version = target, account.Version+1
		w.Accounts[c.Account] = account
		return WorkflowResult{Status: "changed", Version: account.Version}
	case "request-funding":
		if !actor.Finance {
			return result("forbidden")
		}
		amount, ok := new(big.Int).SetString(c.Minor, 10)
		if !ok || !canonicalPositive.MatchString(c.Minor) || amount.Cmp(maximum) > 0 {
			return result("invalid")
		}
		if !exists || account.Tenant != c.Tenant {
			return result("not_found")
		}
		if account.State != "active" {
			return result("inactive")
		}
		if _, exists := w.requests[c.Key]; exists {
			return result("conflict")
		}
		w.requests[c.Key] = workflowApproval{c.Tenant, c.Account, c.Actor, c.Minor, "pending"}
		return result("pending")
	case "approve", "reject", "post-funding":
		request, found := w.requests[c.Reference]
		if !actor.Finance || request.Requester == c.Actor {
			return result("forbidden")
		}
		if !found || request.Tenant != c.Tenant {
			return result("not_found")
		}
		if c.Kind != "post-funding" {
			if request.State != "pending" {
				return result("conflict")
			}
			request.State = map[string]string{"approve": "approved", "reject": "rejected"}[c.Kind]
			w.requests[c.Reference] = request
			return result(request.State)
		}
		if request.State != "approved" || w.Accounts[request.Account].State != "active" {
			return result("conflict")
		}
		clearing := "clearing:" + c.Tenant
		w.Ledger.AddZeroSystem(clearing)
		posted := w.Ledger.Move(Intent{Tenant: c.Tenant, Actor: c.Actor, Operation: "funding", Key: c.Key, From: clearing, To: request.Account, Currency: "INR", Minor: request.Minor})
		if posted.Status == "posted" {
			request.State = "posted"
			w.requests[c.Reference] = request
		}
		return result(posted.Status)
	case "transfer":
		destination, found := w.Accounts[c.To]
		if !exists || !found || account.Tenant != c.Tenant || destination.Tenant != c.Tenant || account.Owner != c.Actor || destination.Owner != c.Actor {
			return result("forbidden")
		}
		if account.State != "active" || destination.State != "active" {
			return result("inactive")
		}
		outcome := w.Ledger.Move(Intent{Tenant: c.Tenant, Actor: c.Actor, Operation: "transfer", Key: c.Key, From: c.Account, To: c.To, Currency: "INR", Minor: c.Minor})
		return WorkflowResult{Status: outcome.Status, Replayed: outcome.Replayed}
	default:
		return result("invalid")
	}
}
