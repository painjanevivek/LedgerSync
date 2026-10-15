FROM golang:1.26.9-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS builder
WORKDIR /src
COPY go.mod go.sum go.work ./
COPY cmd/study-proxy ./cmd/study-proxy
COPY internal/platform/routing ./internal/platform/routing
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/study-proxy ./cmd/study-proxy
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
USER 65532:65532
COPY --from=builder /out/study-proxy /usr/local/bin/study-proxy
ENTRYPOINT ["/usr/local/bin/study-proxy"]
