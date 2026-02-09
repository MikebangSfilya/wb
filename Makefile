-include .env
export

COMPOSE = docker compose
APP = wb-service
BIN = bin/$(APP)
MAIN_PATH = cmd/app/main.go
PROD_PATH = cmd/producer/main.go
DB_URL = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
MIGRATE := $(shell command -v migrate 2> /dev/null)
ifeq ($(MIGRATE),)
    MIGRATE = $(shell go env GOPATH)/bin/migrate
endif


.PHONY: migrate-up migrate-down migrate-create migrate-version \
        tidy build run deps test bench clean \
        compose-build up up-infra down down-full logs \
        db tables dev reset lint prod-run

lint:
	golangci-lint run ./...

migrate-up:
	$(MIGRATE) -path ./db/migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path ./db/migrations -database "$(DB_URL)" down 1

migrate-create:
ifeq ($(strip $(NAME)),)
	@echo "Usage: make migrate-create NAME=some_name"
	@exit 1
endif
	$(MIGRATE) create -ext sql -dir ./db/migrations -seq $(NAME)

migrate-version:
	$(MIGRATE) -path ./db/migrations -database "$(DB_URL)" version
build: deps
	@mkdir -p bin
	go build -o $(BIN) $(MAIN_PATH)

run:
	go run $(MAIN_PATH)

prod-run:
	go run $(PROD_PATH)

deps:
	go mod tidy
	go mod download

test:
	go test -v ./internal/...

bench:
	go test -v -bench=. -benchmem ./internal/...

clean:
	rm -rf bin/

tidy:
	go mod tidy

compose-build:
	$(COMPOSE) build

up:
	$(MAKE) up-infra
	sleep 3
	$(MAKE) prod-run
	$(MAKE) run

up-infra:
	$(COMPOSE) up -d postgres redis kafka jaeger

down:
	$(COMPOSE) down

down-full:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f

db:
	$(COMPOSE) exec postgres psql -U $(DB_USER) $(DB_NAME)

tables:
	$(COMPOSE) exec postgres psql -U $(DB_USER) $(DB_NAME) -c "\dt"

dev: up-infra build
	./$(BIN)

reset: down-full compose-build up