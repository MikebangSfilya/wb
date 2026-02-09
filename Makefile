-include .env
export

COMPOSE = docker compose
APP = wb-service
BIN = bin/$(APP)
MAIN_PATH = cmd/app/main.go
PROD_PATH = cmd/producer/main.go
DB_URL = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: migrate-up migrate-down migrate-create migrate-version \
        tidy build run deps test bench clean \
        compose-build up up-infra down down-full logs \
        db tables dev reset lint prod-run

lint:
	golangci-lint run ./...

migrate-up:
	migrate -path ./db/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path ./db/migrations -database "$(DB_URL)" down 1

migrate-create:
ifeq ($(strip $(NAME)),)
	@echo "Usage: make migrate-create NAME=some_name"
	@exit 1
endif
	migrate create -ext sql -dir ./db/migrations -seq $(NAME)

migrate-version:
	migrate -path ./db/migrations -database "$(DB_URL)" version

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

up: up-infra prod-run run

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