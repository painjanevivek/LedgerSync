FROM golang:1.26.6-alpine AS builder
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reconcile ./cmd/reconcile

FROM alpine:3.24
RUN addgroup -S ledgersync && adduser -S ledgersync -G ledgersync && apk add --no-cache ca-certificates
USER ledgersync
COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/reconcile /usr/local/bin/reconcile
WORKDIR /
COPY migrations /migrations
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
