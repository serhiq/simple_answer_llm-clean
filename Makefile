# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Linting
GOLINT=golangci-lint

all: lint test

build:
	$(GOBUILD) -o evotor-ai ./cmd/evotor-ai

test:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

benchmark:
	$(GOTEST) -v -bench=. -benchmem ./...

examples:
	./evotor-ai --timeout 60 "Сколько чеков за январь"

repl: build
	./evotor-ai

lint:
	$(GOLINT) run

clean:
	$(GOCLEAN)
	rm -f coverage.out
	rm -f coverage.html
	rm -f evotor-ai
	rm -f evotor-ai.log

deps:
	$(GOGET) -v -d ./...
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: all build test benchmark lint clean deps examples repl
