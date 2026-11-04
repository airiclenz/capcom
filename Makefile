# Build capcom. Same recipe as the release tool: static binary, trimmed paths.
# BUILDFLAGS, not GOFLAGS: a make variable named GOFLAGS would replace the go tool's own
# GOFLAGS environment variable in every recipe.
BINARY     ?= capcom
BUILDFLAGS ?=
LDFLAGS    ?=
PKG        ?= ./...

# The linters CI's lint job runs, at the same pins as its env block in .github/workflows/ci.yml.
STATICCHECK_VERSION ?= 2026.2.2
GOVULNCHECK_VERSION ?= v1.9.0

.PHONY: all build vet test lint clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath $(BUILDFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/capcom

vet:
	go vet $(PKG)

test:
	go test -count=1 $(PKG)

lint:
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
