# Build capcom. Same recipe as the release tool: static binary, trimmed paths.
# BUILDFLAGS, not GOFLAGS: a make variable named GOFLAGS would replace the go tool's own
# GOFLAGS environment variable in every recipe.
BINARY     ?= capcom
BUILDFLAGS ?=
LDFLAGS    ?=
PKG        ?= ./...

.PHONY: all build vet test clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath $(BUILDFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/capcom

vet:
	go vet $(PKG)

test:
	go test $(PKG)

clean:
	rm -f $(BINARY) $(BINARY).exe
