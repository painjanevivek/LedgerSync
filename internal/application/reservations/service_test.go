package reservations

import (
	"context"
	"errors"
	"testing"
)

type recordingRepository struct {
	creates   []Create
	decisions []Decide
	snapshot  Snapshot
}

func (r *recordingRepository) Snapshot(context.Context, Scope) (Snapshot, error) {
	return r.snapshot, nil
}

func (r *recordingRepository) Create(_ context.Context, c Create) (Receipt, error) {
	r.creates = append(r.creates, c)
	return Receipt{ID: c.DebitAccountID, State: "active"}, nil
}
func (r *recordingRepository) Decide(_ context.Context, c Decide) (Receipt, error) {
	r.decisions = append(r.decisions, c)
	return Receipt{ID: c.HoldID, State: "released"}, nil
}
func (*recordingRepository) Read(context.Context, Scope, string) (Hold, error) {
	return Hold{}, ErrNotFound
}
func (*recordingRepository) List(context.Context, Scope, string) (Page, error) {
	return Page{Holds: []Hold{}}, nil
}
func validCreateFixture() Create {
	return Create{Meta: Meta{Scope: Scope{TenantID: "11111111-1111-4111-8111-111111111111", ActorSubjectID: "synthetic-operator"}, IdempotencyKey: "synthetic-original-reservation-key", CorrelationID: "22222222-2222-4222-8222-222222222222"}, DebitAccountID: "33333333-3333-4333-8333-333333333333", CreditAccountID: "44444444-4444-4444-8444-444444444444", AmountMinor: "9223372036854775807", Currency: "INR", TTLSeconds: 86400}
}
func TestReservationCreateInput(t *testing.T) {
	if _, err := NewService(nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("nil repository accepted")
	}
	repo := &recordingRepository{}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	valid := validCreateFixture()
	if _, err := service.Create(ctx, valid); err != nil || len(repo.creates) != 1 || repo.creates[0] != valid {
		t.Fatal("exact original intent changed", err)
	}
	for name, alter := range map[string]func(*Create){"zero": func(c *Create) { c.AmountMinor = "0" }, "overflow": func(c *Create) { c.AmountMinor = "9223372036854775808" }, "leading zero": func(c *Create) { c.AmountMinor = "01" }, "fraction": func(c *Create) { c.AmountMinor = "1.00" }, "same account": func(c *Create) { c.CreditAccountID = c.DebitAccountID }, "zero tenant": func(c *Create) { c.TenantID = "00000000-0000-0000-0000-000000000000" }, "actor control": func(c *Create) { c.ActorSubjectID = "operator\n" }, "short key": func(c *Create) { c.IdempotencyKey = "short" }, "lower currency": func(c *Create) { c.Currency = "inr" }, "zero ttl": func(c *Create) { c.TTLSeconds = 0 }, "long ttl": func(c *Create) { c.TTLSeconds = 86401 }} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			alter(&candidate)
			if _, err := service.Create(ctx, candidate); !errors.Is(err, ErrInvalid) {
				t.Fatal("invalid request reached repository", err)
			}
		})
	}
	if len(repo.creates) != 1 {
		t.Fatal("invalid command reached financial adapter")
	}
}
func TestReservationTerminalAndReadBoundary(t *testing.T) {
	repo := &recordingRepository{}
	service, _ := NewService(repo)
	valid := validCreateFixture()
	ctx := context.Background()
	decision := Decide{Meta: valid.Meta, HoldID: valid.DebitAccountID, Decision: "release"}
	if _, err := service.Decide(ctx, decision); err != nil || len(repo.decisions) != 1 || repo.decisions[0] != decision {
		t.Fatal("terminal intent changed", err)
	}
	for _, action := range []string{"expire", "partial_capture", ""} {
		candidate := decision
		candidate.Decision = action
		if _, err := service.Decide(ctx, candidate); !errors.Is(err, ErrInvalid) {
			t.Fatal("unselected decision reached repository")
		}
	}
	if _, err := service.Read(ctx, valid.Scope, valid.DebitAccountID); !errors.Is(err, ErrNotFound) {
		t.Fatal("repository denial changed", err)
	}
	if _, err := service.Read(ctx, Scope{}, valid.DebitAccountID); !errors.Is(err, ErrInvalid) {
		t.Fatal("unscoped read accepted")
	}
	if _, err := service.List(ctx, valid.Scope, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(ctx, valid.Scope, string(make([]byte, 257))); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded cursor accepted")
	}
}
