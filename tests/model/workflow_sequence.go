package model

import (
	"fmt"
	"math/rand"
)

func GenerateWorkflow(seed int64, count int, tenant, operator, reviewer string) []WorkflowCommand {
	if count < 20 || count > 128 {
		panic("workflow histories require 20-128 commands")
	}
	w := NewWorkflow()
	w.Grant(operator, tenant, true)
	w.Grant(reviewer, tenant, true)
	commands := []WorkflowCommand{}
	add := func(command WorkflowCommand) {
		if command.Tenant == "" {
			command.Tenant = tenant
		}
		if command.Actor == "" {
			command.Actor = operator
		}
		command.Key = fmt.Sprintf("workflow-%d-step-%03d", seed, len(commands))
		commands = append(commands, command)
		w.Apply(command)
	}
	add(WorkflowCommand{Kind: "create", Account: "a"})
	add(WorkflowCommand{Kind: "create", Account: "b"})
	add(WorkflowCommand{Kind: "request-funding", Account: "a", Minor: "100"})
	request := commands[2].Key
	add(WorkflowCommand{Kind: "approve", Reference: request})
	add(WorkflowCommand{Kind: "approve", Actor: reviewer, Reference: request})
	add(WorkflowCommand{Kind: "reject", Actor: reviewer, Reference: request})
	add(WorkflowCommand{Kind: "post-funding", Actor: reviewer, Reference: request})
	add(WorkflowCommand{Kind: "transfer", Account: "a", To: "b", Minor: "10"})
	add(WorkflowCommand{Kind: "freeze", Account: "a", ExpectedVersion: 1})
	add(WorkflowCommand{Kind: "transfer", Account: "a", To: "b", Minor: "10"})
	add(WorkflowCommand{Kind: "reactivate", Account: "a", ExpectedVersion: 2})
	add(WorkflowCommand{Kind: "close", Account: "a", ExpectedVersion: 3})
	add(WorkflowCommand{Kind: "create", Account: "c"})
	add(WorkflowCommand{Kind: "close", Account: "c", ExpectedVersion: 1})
	add(WorkflowCommand{Kind: "reactivate", Account: "c", ExpectedVersion: 2})
	add(WorkflowCommand{Kind: "create", Actor: "ungranted-workflow-actor", Account: "denied"})
	add(WorkflowCommand{Kind: "create", Tenant: "00000000-0000-4000-8000-000000000777", Account: "foreign"})
	add(WorkflowCommand{Kind: "freeze", Account: "a", ExpectedVersion: 1})
	commands = append(commands, commands[8]) // Exact old receipt after state changed.
	random := rand.New(rand.NewSource(seed))
	for len(commands) < count {
		account := []string{"a", "b", "c"}[random.Intn(3)]
		version := w.Accounts[account].Version
		if len(commands)%4 == 0 {
			version = 1
		}
		add(WorkflowCommand{Kind: []string{"freeze", "reactivate", "close"}[random.Intn(3)], Account: account, ExpectedVersion: version})
	}
	return commands
}
