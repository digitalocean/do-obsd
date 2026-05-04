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

# Overridable by the release pipeline (VERSION=1.2.3 make build). Default keeps
# current dev UX (e.g. v0.1.2-3-gabc-dirty). Strip a leading 'v' so the in-binary
# version matches the deb/rpm package version format that
# packaging/scripts/update.sh already parses. Falls back to "dev" outside a git
# checkout (e.g. when building from an extracted source tarball).
VERSION ?= $(or $(patsubst v%,%,$(shell git describe --tags --always --dirty 2>/dev/null)),dev)

ldflags = -s -w -X main.version=$(VERSION)

.PHONY: build test lint mocks

build:
	$(print)
	CGO_ENABLED=0 go build -trimpath -ldflags '$(ldflags)' -o bin/do-obsd ./cmd/do-obsd

test:
	$(print)
	go test -race -cover ./...

lint:
	$(print)
	$(shellcheck) $(shell find . -type f -name "*.sh" ! -path "./vendor/*")
	$(linter) ./...

mocks:
	$(print)
	$(mockgen) -source=internal/supervisor/os.go -package=supervisor -destination=internal/supervisor/mocks_os_test.go
