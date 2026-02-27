# Multi-stage Dockerfile for building portainer-mcp from source
# For release images, GoReleaser uses Dockerfile.goreleaser instead.

FROM golang:1.24-alpine AS builder
RUN apk --no-cache add ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
    -o /portainer-mcp ./cmd/portainer-mcp

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /portainer-mcp /usr/local/bin/portainer-mcp
COPY tools.yaml /tools.yaml
ENTRYPOINT ["/usr/local/bin/portainer-mcp"]
