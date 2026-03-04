.PHONY: generate build run migrate docker-up docker-down

generate:
	mkdir -p pkg/generated
	oapi-codegen -package generated -generate types -o pkg/generated/types.gen.go api/openapi.yaml
	oapi-codegen -package generated -generate chi-server -o pkg/generated/server.gen.go api/openapi.yaml

build: generate
	go build -o bin/app ./cmd/app

run: generate
	go run ./cmd/app

migrate:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/marketplace?sslmode=disable" up

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down -v