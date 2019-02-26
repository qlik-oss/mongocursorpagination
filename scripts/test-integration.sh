#!/usr/bin/env sh

# Purpose: This script runs the integration tests.
# Instructions: make test-integration <ARGS>

set -eu

ARGS=$1

# The idiomatic way to disable test caching explicitly is to use -count=1
go test -count=1 -race ./test/integration $ARGS
