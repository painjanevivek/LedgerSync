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

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/openingimports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func main() {
	action := flag.String("action", "validate", "validate, request, approve, or execute")
	file := flag.String("manifest", "", "reviewed opening import JSON manifest")
	pilot := flag.String("pilot-currency", "", "selected pilot currency")
	actor := flag.String("actor-subject-id", "", "strong finance actor")
	correlation := flag.String("correlation-id", "", "operation correlation UUID")
	flag.Parse()
	if !slices.Contains([]string{"validate", "request", "approve", "execute"}, *action) {
		fail(fmt.Errorf("unsupported action %q", *action))
	}
	if *file == "" || *pilot == "" {
		fail(fmt.Errorf("a reviewed manifest and selected pilot currency are required"))
	}
	content, err := os.ReadFile(*file)
	if err != nil {
		fail(err)
	}
	manifest, err := openingimports.DecodeManifest(bytes.NewReader(content))
	if err != nil {
		fail(err)
	}
	prepared, err := manifest.Validate(context.Background(), *pilot)
	if err != nil {
		fail(err)
	}
	hash := hex.EncodeToString(prepared.ContentHash[:])
	if *action == "validate" {
		fmt.Printf("content_sha256=%s row_count=%d total_minor=%d\n", hash, len(prepared.Rows), prepared.TotalMinor)
		return
	}
	if *actor == "" || *correlation == "" {
		fail(fmt.Errorf("request, approve, and execute require actor and correlation IDs"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: os.Getenv("LEDGERSYNC_DATABASE_URL")})
	if err != nil {
		fail(err)
	}
	defer func() { _ = database.Close() }()
	repository, err := db.NewOpeningImportRepository(database, nil)
	if err != nil {
		fail(err)
	}
	var result openingimports.Result
	switch *action {
	case "request":
		result, err = repository.Request(ctx, prepared, *actor, *correlation)
	case "approve":
		result, err = repository.Approve(ctx, prepared, *actor, *correlation)
	case "execute":
		result, err = repository.Execute(ctx, prepared, *actor, *correlation)
	}
	if err != nil {
		fail(err)
	}
	fmt.Printf("action=%s content_sha256=%s replayed=%t\n", *action, hash, result.Replayed)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
