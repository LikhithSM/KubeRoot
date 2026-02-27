FROM golang:1.25 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build backend only - pure Go, no CGO needed
# CGO=0 disables C bindings (pq driver works fine without it)
# Let Go auto-detect GOOS/GOARCH from base image
RUN CGO_ENABLED=0 go build -o kuberoot ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/kuberoot /usr/local/bin/kuberoot

EXPOSE 8080

CMD ["kuberoot"]
