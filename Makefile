print = @printf ":::::::::::::::: [$(shell date -u)] $@ ::::::::::::::::\n"

shellcheck = docker run --rm \
	-v "$(CURDIR):$(CURDIR)" \
	-w "$(CURDIR)" \
	-u $(shell id -u) \
	koalaman/shellcheck:latest

linter = docker run --rm \
	-v "$(CURDIR):$(CURDIR)" \
	-w "$(CURDIR)" \
	-e "XDG_CACHE_HOME=$(CURDIR)/target/.cache/go" \
	-u $(shell id -u) \
	golangci/golangci-lint:latest \
	golangci-lint run

mockgen = go tool mockgen

.PHONY: build test lint mocks

build:
	$(print)
	CGO_ENABLED=0 go build -o bin/do-obsd ./cmd/do-obsd

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
	$(mockgen) -source=internal/collector/exec.go -package=collector -destination=internal/collector/mocks_exec_test.go
