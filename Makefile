print = @printf ":::::::::::::::: [$(shell date -u)] $@ ::::::::::::::::\n"

shellcheck = docker run --rm \
	-v "$(CURDIR):$(CURDIR)" \
	-w "$(CURDIR)" \
	-u $(shell id -u) \
	koalaman/shellcheck:v0.11.0

linter = docker run --rm \
	-v "$(CURDIR):$(CURDIR)" \
	-w "$(CURDIR)" \
	-e "XDG_CACHE_HOME=$(CURDIR)/target/.cache/go" \
	-u $(shell id -u) \
	golangci/golangci-lint:latest \
	golangci-lint run

mockgen = go tool mockgen

.PHONY: build build-insights-otlp-hosts build-linux-binaries test lint mocks

build:
	$(print)
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(shell git describe --tags --always --dirty)" -o bin/do-obsd ./cmd/do-obsd

build-insights-otlp-hosts:
	$(print)
	CGO_ENABLED=0 go build -trimpath -o bin/insights-otlp-hosts ./cmd/insights-otlp-hosts

build-linux-binaries:
	$(print)
	VERSION=$(shell git describe --tags --always --dirty) sh packaging/scripts/build-linux-binaries.sh

test:
	$(print)
	go test -race -cover ./...

lint:
	$(print)
	$(shellcheck) $(shell find . -type f -name "*.sh" ! -path "./vendor/*")
	$(linter) ./...

mocks:
	$(print)
	$(mockgen) -source=internal/collector/os.go -package=collector -destination=internal/collector/mocks_os_test.go
