# Makefile for OmniRAN-Emulator
GO := $(shell which go 2>/dev/null || echo "./go_sdk/bin/go")

.PHONY: all build clean docker-build docker-up docker-down

all: build

build:
	$(GO) build -o app cmd/app.go

clean:
	rm -f app

docker-build:
	docker build -f docker/Dockerfile --target my5grantester --tag omniran-emulator:latest .

docker-up:
	docker-compose -f docker/docker-compose.yml up -d

docker-down:
	docker-compose -f docker/docker-compose.yml down
