SHELL=/usr/bin/env bash
PROJECTNAME=$(shell basename "$(PWD)")

## help: Get more info on make commands.
help: Makefile
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
.PHONY: help

## build: Build unison binary.
build:
	@echo "--> Building Unison"
	@go build -o build/ ./unison-poc
.PHONY: build

## clean: Clean up binary.
clean:
	@echo "--> Cleaning up ./build"
	@rm -rf build/*
.PHONY: clean

## lint: Linting *.go files using golangci-lint. Look for .golangci.yml for the list of linters.
lint:
	@echo "--> Running linter"
	@golangci-lint run
.PHONY: lint
