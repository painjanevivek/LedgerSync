FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd ./cmd
COPY contracts ./contracts
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reconcile ./cmd/reconcile

FROM postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777 AS database-tooling
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
ENTRYPOINT ["/usr/local/bin/api"]
