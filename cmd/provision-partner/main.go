package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/provisioning"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func main() {
	action := flag.String("action", "validate", "validate, apply, or rollback")
	file := flag.String("config", "", "reviewed JSON config")
	pilot := flag.String("pilot-currency", "", "selected pilot currency")
	actor := flag.String("actor-subject-id", "", "trusted operator")
	correlation := flag.String("correlation-id", "", "change UUID")
	tenant := flag.String("tenant-id", "", "tenant UUID for rollback")
	flag.Parse()
	if !slices.Contains([]string{"validate", "apply", "rollback"}, *action) {
		fail(fmt.Errorf("unsupported action %q", *action))
	}
	if *action == "rollback" {
		if *tenant == "" || *actor == "" || *correlation == "" {
			fail(fmt.Errorf("rollback requires tenant ID, trusted actor, and correlation ID"))
		}
	} else if *file == "" || *pilot == "" {
		fail(fmt.Errorf("validate/apply require a reviewed config and selected pilot currency"))
	} else if *action == "apply" && (*actor == "" || *correlation == "") {
		fail(fmt.Errorf("apply requires a trusted actor and correlation ID"))
	}
	var configuration provisioning.Config
	if *action != "rollback" {
		content, err := os.ReadFile(*file)
		if err != nil {
			fail(err)
		}
		configuration, err = provisioning.DecodeConfig(bytes.NewReader(content))
		if err != nil {
			fail(err)
		}
		fingerprint, err := configuration.Validate(*pilot)
		if err != nil {
			fail(err)
		}
		if *action == "validate" {
			fmt.Println(hex.EncodeToString(fingerprint[:]))
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: os.Getenv("LEDGERSYNC_DATABASE_URL")})
	if err != nil {
		fail(err)
	}
	defer func() { _ = database.Close() }()
	repository, err := db.NewProvisioningRepository(database, nil)
	if err != nil {
		fail(err)
	}
	switch *action {
	case "apply":
		err = repository.Apply(ctx, configuration, *pilot, *actor, *correlation)
	case "rollback":
		err = repository.Rollback(ctx, *tenant, *actor, *correlation)
	}
	if err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
