.PHONY: format lint test build run-api run-worker migrate reconcile compose-up compose-down

format:
	gofmt -w cmd internal
	cd web && npm run lint -- --fix

lint:
	go vet ./cmd/... ./internal/...
	cd web && npm run lint

test:
	go test ./cmd/... ./internal/...
	cd web && npm run build

build:
	docker build -f deploy/docker/api.Dockerfile -t ledgersync-api:local .
	docker build -f deploy/docker/outbox-worker.Dockerfile -t ledgersync-worker:local .
	docker build -f deploy/docker/web.Dockerfile -t ledgersync-web:local .

run-api:
	docker compose -f deploy/compose/docker-compose.yml up --build api

run-worker:
	docker compose -f deploy/compose/docker-compose.yml up --build outbox-worker

migrate:
	go run ./cmd/migrate

reconcile:
	go run ./cmd/reconcile

compose-up:
	docker compose -f deploy/compose/docker-compose.yml up --build

compose-down:
	docker compose -f deploy/compose/docker-compose.yml down
