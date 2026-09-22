.PHONY: help docs swagger swag-install dev build test clean

help:
	@echo "Smart Grid Asset Management backend - available targets:"
	@echo "  make docs         - Regenerate Swagger documentation (alias: swagger)"
	@echo "  make swag-install - Install the swag CLI"
	@echo "  make dev          - Run the server locally"
	@echo "  make build        - Build the server binary"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Remove build artifacts"

docs: swagger

swagger:
	@echo "Regenerating Swagger documentation..."
	@swag init -g main.go -o docs --parseInternal
	@echo "Swagger documentation regenerated at ./docs (served at /swagger/index.html)."

swag-install:
	go install github.com/swaggo/swag/cmd/swag@latest

dev:
	go run .

build:
	go build -o bin/server .

test:
	go test ./...

clean:
	rm -rf bin
	go clean -cache -testcache
