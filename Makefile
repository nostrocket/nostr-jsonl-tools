# Makefile for Nostr data processing toolkit

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary output directory
BINDIR=bin

# List of all commands with Go files
COMMANDS = \
	content-extractor \
	dedup \
	dgraph-follow \
	dgraph-server \
	event-filter \
	event-radix-sorter \
	event-splitter \
	extract-pubkeys \
	follow-graph \
	line-counter

# Default target
all: clean setup $(COMMANDS)

# Setup target to create bin directory
setup:
	mkdir -p $(BINDIR)

# Clean target
clean:
	$(GOCLEAN)
	rm -rf $(BINDIR)

# Target to build all binaries
$(COMMANDS):
	@echo "Building $@..."
	$(GOBUILD) -o $(BINDIR)/$@ ./cmd/$@

# Target to build a specific binary
.PHONY: $(COMMANDS)

# Target to run tests
test:
	$(GOTEST) -v ./...

# Target to update dependencies
deps:
	$(GOMOD) tidy

# Target to install binaries to GOPATH/bin
install: all
	cp $(BINDIR)/* $(GOPATH)/bin/

# Help target
help:
	@echo "Nostr Data Processing Toolkit Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build all binaries"
	@echo "  make <command>    Build a specific binary (e.g., make dgraph-follow)"
	@echo "  make clean        Remove all binaries"
	@echo "  make test         Run tests"
	@echo "  make deps         Update dependencies"
	@echo "  make install      Install binaries to GOPATH/bin"
	@echo ""
	@echo "Available commands:"
	@for cmd in $(COMMANDS); do \
		echo "  $$cmd"; \
	done

.PHONY: all setup clean test deps install help
