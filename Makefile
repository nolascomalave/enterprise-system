SHELL := /bin/bash
SWAG_VERSION ?= v1.16.4

.PHONY: run test vet swagger tidy

run:
	go run ./cmd/api

test:
	go test -count=1 ./...

vet:
	go vet ./...

swagger:
	go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init \
		-g cmd/api/main.go -o docs --parseDependency --parseInternal

tidy:
	go mod tidy
