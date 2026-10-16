# Build capcom. Same recipe as the release tool: static binary, trimmed paths.
BINARY   := capcom
LDFLAGS  :=
GOFLAGS  :=

.PHONY: all build vet test clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/capcom

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
