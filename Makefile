# GOWORK=off keeps this module self-contained even when it lives inside a
# parent Go workspace during development. It is a harmless no-op for a
# standalone checkout (e.g. a fresh git clone).
export GOWORK=off

BINARY := bin/shortlink

.PHONY: run build test test-race lint tidy clean docker

run: ## Run the server (DB at ./shortlink.db)
	go run ./cmd/server

build: ## Compile the binary into ./bin
	go build -o $(BINARY) ./cmd/server

test: ## Run the test suite
	go test ./... -count=1

test-race: ## Run tests with the race detector
	go test ./... -race -count=1

lint: ## Vet and check formatting
	go vet ./...
	gofmt -l .

tidy: ## Tidy module dependencies
	go mod tidy

clean: ## Remove build artifacts and the local database
	rm -rf bin shortlink.db shortlink.db-wal shortlink.db-shm

docker: ## Build the Docker image
	docker build -t shortlink:latest .
