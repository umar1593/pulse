.PHONY: help up down logs psql migrate-up migrate-down migrate-status run build test test-integration lint loadgen clean

POSTGRES_DSN ?= postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

up: ## start postgres
	docker compose up -d

down: ## stop everything
	docker compose down

logs: ## tail postgres logs
	docker compose logs -f postgres-primary

psql: ## open psql shell on primary
	docker compose exec postgres-primary psql -U pulse -d pulse

migrate-up: ## apply pending migrations
	goose -dir migrations postgres "$(POSTGRES_DSN)" up

migrate-down: ## roll back the last migration
	goose -dir migrations postgres "$(POSTGRES_DSN)" down

migrate-status: ## show migration state
	goose -dir migrations postgres "$(POSTGRES_DSN)" status

run: ## run ingest-api locally
	go run ./cmd/ingest-api

build: ## build all binaries
	mkdir -p bin && go build -o bin/ ./cmd/...

test: ## unit tests
	go test -race -count=1 ./...

test-integration: ## integration tests (requires docker)
	go test -race -count=1 -tags=integration ./...

lint: ## golangci-lint
	golangci-lint run ./...

loadgen: ## python load generator (1k rps for 60s)
	python3 tools/loadgen.py --rps 1000 --duration 60s

clean:
	rm -rf bin/
	docker compose down -v
