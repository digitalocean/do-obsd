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

# Overridable by the release pipeline (VERSION=1.2.3 make build). Default falls
# back to git describe (e.g. 0.1.2-3-gabc) or to "dev" outside a git checkout.
# Leading 'v' stripped from any source to match the deb/rpm package version format.
override VERSION := $(patsubst v%,%,$(or $(VERSION),$(shell git describe --tags --always 2>/dev/null),dev))

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
