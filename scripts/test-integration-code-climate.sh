#!/usr/bin/env sh

set -eu

ARGS=$1

go test -coverprofile=integration.cover ./test/integration $ARGS
