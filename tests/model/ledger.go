// Package model is an independent test oracle. It uses only the Go standard
// library. It does not call production money, fingerprint or posting code.
package model

import (
	"fmt"
	"math/big"
	"regexp"
)

type Account struct {
	Minor   string
	Version uint64
	System  bool
}
type Intent struct{ Tenant, Actor, Operation, Key, From, To, Currency, Minor string }
type Outcome struct {
	Status   string
	Replayed bool
	Journal  int
	From, To Account
}
type requestKey struct{ Tenant, Actor, Operation, Key string }
type stored struct {
	Intent  Intent
	Outcome Outcome
}
type Ledger struct {
	Accounts map[string]Account
	Journals int
	requests map[requestKey]stored
	total    *big.Int
	currency string
}

var canonicalPositive = regexp.MustCompile(`^[1-9][0-9]*$`)
var maximum = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 63), big.NewInt(1))
var minimum = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 63))

func New(accounts map[string]Account) *Ledger {
	return NewForCurrency(accounts, "USD")
}

// NewForCurrency selects one currency for the complete independent fixture.
// New keeps USD so archived seeds and replay inputs retain their meaning.
func NewForCurrency(accounts map[string]Account, currency string) *Ledger {
	if currency != "USD" && currency != "INR" {
		panic("unsupported model fixture currency")
	}
	ledger := &Ledger{Accounts: map[string]Account{}, requests: map[requestKey]stored{}, total: new(big.Int), currency: currency}
	for id, account := range accounts {
		ledger.Accounts[id] = account
		value, ok := new(big.Int).SetString(account.Minor, 10)
		if !ok {
			panic("invalid model fixture")
		}
		ledger.total.Add(ledger.total, value)
	}
	return ledger
}
func (l *Ledger) AddZeroSystem(id string) {
	if _, exists := l.Accounts[id]; !exists {
		l.Accounts[id] = Account{Minor: "0", System: true}
	}
}

// Move selects an outcome before the database is called. A rejected intent is
// also durable. Later funding cannot turn its replay into another movement.
func (l *Ledger) Move(intent Intent) Outcome {
	key := requestKey{intent.Tenant, intent.Actor, intent.Operation, intent.Key}
	if saved, ok := l.requests[key]; ok {
		if saved.Intent != intent {
			return Outcome{Status: "conflict"}
		}
		result := saved.Outcome
		result.Replayed = true
		return result
	}
	from, fromOK := l.Accounts[intent.From]
	to, toOK := l.Accounts[intent.To]
	amount, ok := new(big.Int).SetString(intent.Minor, 10)
	if !ok || !canonicalPositive.MatchString(intent.Minor) || amount.Cmp(maximum) > 0 || !fromOK || !toOK || intent.From == intent.To || intent.Currency != l.currency || intent.Tenant == "" || intent.Actor == "" || intent.Operation == "" || intent.Key == "" {
		return Outcome{Status: "invalid"}
	}
	source, _ := new(big.Int).SetString(from.Minor, 10)
	destination, _ := new(big.Int).SetString(to.Minor, 10)
	source.Sub(source, amount)
	destination.Add(destination, amount)
	result := Outcome{Status: "posted", From: from, To: to}
	if !from.System && source.Sign() < 0 {
		result.Status = "rejected"
	} else if source.Cmp(minimum) < 0 || source.Cmp(maximum) > 0 || destination.Cmp(minimum) < 0 || destination.Cmp(maximum) > 0 {
		result.Status = "overflow"
	}
	if result.Status == "posted" {
		from.Minor = source.String()
		to.Minor = destination.String()
		from.Version++
		to.Version++
		l.Accounts[intent.From] = from
		l.Accounts[intent.To] = to
		l.Journals++
		result.Journal = l.Journals
		result.From = from
		result.To = to
	}
	l.requests[key] = stored{Intent: intent, Outcome: result}
	return result
}

func (l *Ledger) Check() error {
	total := new(big.Int)
	for id, account := range l.Accounts {
		amount, ok := new(big.Int).SetString(account.Minor, 10)
		if !ok || amount.Cmp(minimum) < 0 || amount.Cmp(maximum) > 0 || !account.System && amount.Sign() < 0 {
			return fmt.Errorf("invalid model account %s", id)
		}
		total.Add(total, amount)
	}
	if total.Cmp(l.total) != 0 {
		return fmt.Errorf("model conservation failed: %s != %s", total, l.total)
	}
	return nil
}
