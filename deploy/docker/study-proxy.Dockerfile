FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd/study-proxy ./cmd/study-proxy
COPY internal/platform/routing ./internal/platform/routing
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/study-proxy ./cmd/study-proxy
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
USER 65532:65532
COPY --from=builder /out/study-proxy /usr/local/bin/study-proxy
ENTRYPOINT ["/usr/local/bin/study-proxy"]
