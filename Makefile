SERVER := server
COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: lint lint-fix test build setup-env up down logs

lint:
	@unformatted="$$(cd $(SERVER) && gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd $(SERVER) && go vet ./...

lint-fix:
	cd $(SERVER) && gofmt -w .

test:
	cd $(SERVER) && go test ./...

build:
	cd $(SERVER) && go build -o bin/ ./cmd/...

setup-env:
	@test -f deploy/.env || (cp deploy/.env.example deploy/.env && echo "Created deploy/.env from .env.example")

up: setup-env
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f
