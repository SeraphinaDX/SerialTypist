GO ?= go
BINARY ?= serialtypist

.PHONY: all build test vet run clean

all: test build

build:
	$(GO) build -buildvcs=false -o $(BINARY) ./cmd/serialtypist

test:
	$(GO) test -buildvcs=false ./...

vet:
	$(GO) vet -buildvcs=false ./...

run:
	$(GO) run -buildvcs=false ./cmd/serialtypist

clean:
	rm -f $(BINARY)
