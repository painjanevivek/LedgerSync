FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
RUN addgroup -S ledgersync && adduser -S ledgersync -G ledgersync && apk add --no-cache ca-certificates
USER ledgersync
COPY --from=builder /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
