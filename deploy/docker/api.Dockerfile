FROM golang:1.26.9-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS builder
ARG GO_BUILD_GODEBUG=""
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd ./cmd
COPY contracts ./contracts
COPY internal ./internal
RUN GODEBUG="$GO_BUILD_GODEBUG" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN GODEBUG="$GO_BUILD_GODEBUG" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN GODEBUG="$GO_BUILD_GODEBUG" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reconcile ./cmd/reconcile
RUN GODEBUG="$GO_BUILD_GODEBUG" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reconciliation-scheduler ./cmd/reconciliation-scheduler

FROM postgres:16-alpine@sha256:721873c34ceb9f8d8fc265984940dc982404c105f19ad51be9fdc5970a6080ea AS database-tooling
RUN apk upgrade --no-cache && rm -f /usr/local/bin/gosu
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/reconcile /usr/local/bin/reconcile
WORKDIR /
COPY migrations /migrations
COPY deploy/postgres /database-roles
USER postgres

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates \
    && addgroup -S ledgersync \
    && adduser -S ledgersync -G ledgersync
USER ledgersync
COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/reconcile /usr/local/bin/reconcile
WORKDIR /
COPY migrations /migrations
EXPOSE 8080
COPY --from=builder /out/reconciliation-scheduler /usr/local/bin/reconciliation-scheduler
ENTRYPOINT ["/usr/local/bin/api"]
