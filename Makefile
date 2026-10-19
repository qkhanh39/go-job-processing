.PHONY: up down logs migrate-up migrate-create migrate-down reset-db

include .env
export $(shell sed 's/=.*//' .env)

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate-up:
	migrate -path db_migrations \
		-database "postgresql://$(DB_SOURCE_USERNAME):$(DB_SOURCE_PASSWORD)@localhost:$(DB_SOURCE_PORT)/$(DB_SOURCE_DATABASE)?sslmode=$(DB_SOURCE_SSL_MODE)" \
		up

migrate-create:
	migrate create -ext sql -dir db_migrations -seq $(name)

migrate-down:
	migrate -path db_migrations \
		-database "postgresql://$(DB_SOURCE_USERNAME):$(DB_SOURCE_PASSWORD)@localhost:$(DB_SOURCE_PORT)/$(DB_SOURCE_DATABASE)?sslmode=$(DB_SOURCE_SSL_MODE)" \
		down

reset-db:
	docker compose down -v
	docker compose up -d postgres
	sleep 3
	$(MAKE) migrate-up