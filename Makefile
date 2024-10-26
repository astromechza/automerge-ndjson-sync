# Disable all the default make stuff
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:

## Display a list of the documented make targets
.PHONY: help
help:
	@echo Documented Make targets:
	@perl -e 'undef $$/; while (<>) { while ($$_ =~ /## (.*?)(?:\n# .*)*\n.PHONY:\s+(\S+).*/mg) { printf "\033[36m%-30s\033[0m %s\n", $$2, $$1 } }' $(MAKEFILE_LIST) | sort

# ------------------------------------------------------------------------------
# NON-PHONY TARGETS
# ------------------------------------------------------------------------------

.PHONY: .FORCE
.FORCE:

# ------------------------------------------------------------------------------
# PHONY TARGETS
# ------------------------------------------------------------------------------

## Execute Go tests with gotestsum (go install gotest.tools/gotestsum@latest)
.PHONY: test
test:
	gotestsum --format testname

## Lint Go code with golangci-lint (install this if you don't have it)
.PHONY: lint
lint:
	go vet ./...
	golangci-lint run --show-stats
