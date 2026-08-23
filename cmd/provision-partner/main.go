package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/provisioning"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"os"
	"time"
)

func main() {
	action := flag.String("action", "validate", "validate, apply, or rollback")
	file := flag.String("config", "", "reviewed JSON config")
	pilot := flag.String("pilot-currency", "", "selected pilot currency")
	actor := flag.String("actor-subject-id", "", "trusted operator")
	correlation := flag.String("correlation-id", "", "change UUID")
	tenant := flag.String("tenant-id", "", "tenant UUID for rollback")
	flag.Parse()
	var configuration provisioning.Config
	if *action != "rollback" {
		content, err := os.ReadFile(*file)
		if err != nil {
			fail(err)
		}
		if err = json.Unmarshal(content, &configuration); err != nil {
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
	defer database.Close()
	repository, err := db.NewProvisioningRepository(database, nil)
	if err != nil {
		fail(err)
	}
	if *action == "apply" {
		err = repository.Apply(ctx, configuration, *pilot, *actor, *correlation)
	} else if *action == "rollback" {
		err = repository.Rollback(ctx, *tenant, *actor, *correlation)
	} else {
		err = fmt.Errorf("unsupported action %q", *action)
	}
	if err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
