FROM golang:1.26.6-alpine AS builder
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/outbox-worker ./cmd/outbox-worker

FROM alpine:3.20
RUN addgroup -S ledgersync && adduser -S ledgersync -G ledgersync && apk add --no-cache ca-certificates
USER ledgersync
COPY --from=builder /out/outbox-worker /usr/local/bin/outbox-worker
ENTRYPOINT ["/usr/local/bin/outbox-worker"]
