# Makefile for the llamanator application
lint:
	@echo "Linting..."
	go fmt ./...
	go vet ./...
	golint ./...
	@echo "Done linting."

test:
	@echo "Testing..."
	go test ./...
	@echo "Done testing."

build:
	@echo "Building..."
	go build -o build/llamanator
	@echo "Done building."

install:
	@echo "Installing..."
	chmod +x build/llamanator
	cp build/llamanator /usr/local/bin/llamanator
	@echo "Installed to /usr/local/bin/llamanator"
