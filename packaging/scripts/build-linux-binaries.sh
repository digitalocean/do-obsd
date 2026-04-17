#!/bin/sh
# Build Linux amd64 binaries in the repository root (same layout as CI before fpm).
# Usage: VERSION=1.2.3 ./packaging/scripts/build-linux-binaries.sh
set -eu

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "${ROOT}"

VERSION=${VERSION:-dev}

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
	-ldflags "-s -w -X main.version=${VERSION}" \
	-o do-obsd ./cmd/do-obsd
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
	-ldflags "-s -w" \
	-o insights-otlp-hosts ./cmd/insights-otlp-hosts
chmod +x do-obsd insights-otlp-hosts
echo "Built ${ROOT}/do-obsd and ${ROOT}/insights-otlp-hosts"
