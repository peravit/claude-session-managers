BINARY  = claude-manager
BIN_DIR = ./bin

.PHONY: build install test clean

build:
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/claude-manager

install:
	go install ./cmd/claude-manager

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)
