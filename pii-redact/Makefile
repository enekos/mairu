.PHONY: build test lint vet install clean tidy

BIN := bin/pii-redact

build:
	@mkdir -p bin
	go build -o $(BIN) ./cmd/pii-redact

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

lint: vet

install:
	go install ./cmd/pii-redact

tidy:
	go mod tidy

clean:
	rm -rf bin
