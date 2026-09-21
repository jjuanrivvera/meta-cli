BINARY := meta
MODULE := github.com/jjuanrivvera/meta-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT) -X $(MODULE)/internal/version.Date=$(DATE)
COVERAGE_MIN ?= 80
API_COVERAGE_MIN ?= 90

.DEFAULT_GOAL := build

build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/$(BINARY)
install:
	CGO_ENABLED=0 go install -ldflags '$(LDFLAGS)' ./cmd/$(BINARY)
uninstall:
	rm -f "$$(go env GOPATH)/bin/$(BINARY)"
run: build
	./bin/$(BINARY) $(ARGS)
dev: format vet build

format:
	gofmt -s -w cmd commands internal tools
fmt:
	@test -z "$$(gofmt -l cmd commands internal tools)" || { gofmt -l cmd commands internal tools; exit 1; }
vet:
	go vet ./...
lint:
	golangci-lint run ./...
security:
	gosec -quiet ./...
	govulncheck ./...
tidy:
	go mod tidy
test:
	go test -count=1 ./...
test-race:
	go test -race -count=1 ./...
test-coverage:
	go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...
cover-check: test-coverage
	./scripts/cover-check.sh $(COVERAGE_MIN)
e2e:
	go test -count=1 -tags=e2e ./e2e -v
docs-check:
	$(MAKE) docs-gen
	git diff --exit-code -- docs/commands
check: fmt vet lint security test docs-check

spec-check:
	./scripts/spec-check.sh
spec-completeness:
	./scripts/spec-completeness.sh api-manifest.json $(API_COVERAGE_MIN)
verify: check spec-check spec-completeness cover-check e2e
	./scripts/dod-check.sh $(BINARY)
	./scripts/judge.sh
judge:
	./scripts/judge.sh
accept: verify

docs-gen:
	go run -tags docsgen ./tools/gendocs
docs-serve:
	mkdocs serve
docs-build:
	mkdocs build --strict
snapshot:
	goreleaser release --snapshot --clean --skip=sign,sbom,docker
setup-hooks:
	git config core.hooksPath .githooks
clean:
	rm -rf bin dist coverage.out site

.PHONY: build install uninstall run dev format fmt vet lint security tidy test test-race \
	test-coverage cover-check e2e docs-check check spec-check spec-completeness verify \
	judge accept docs-gen docs-serve docs-build snapshot setup-hooks clean
