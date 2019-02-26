#!/usr/bin/env sh

set -eu

ARGS=$1

go test -race -coverprofile=integration.cover -coverpkg=github.com/qlik-trial/collections/pkg/messaging,github.com/qlik-trial/collections/internal/collections/store,github.com/qlik-trial/collections/internal/items/store ./test/integration $ARGS
