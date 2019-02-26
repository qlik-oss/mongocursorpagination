#!/usr/bin/env sh

# Purpose: This script runs the unit tests.
# Instructions: make test-unit

set -eu

GO_PACKAGES=$(go list ./... | grep -v "test/")

# The idiomatic way to disable test caching explicitly is to use -count=1
go test -count=1 -race -cover $GO_PACKAGES
