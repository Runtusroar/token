.PHONY: dev deps migrate seed build test down

deps:
	docker-compose up -d postgres redis

dev: deps
	cd backend && go run cmd/server/main.go

seed: deps
	docker-compose exec -T postgres psql -U relay -d relay < backend/migration/seed.sql

build:
	cd backend && go build -o ../bin/relay cmd/server/main.go

test:
	cd backend && go test ./... -v

down:
	docker-compose down
