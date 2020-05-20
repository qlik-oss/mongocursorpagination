#!/usr/bin/env sh

# Purpose: This script lints the code.
# Instructions: make lint

VERSION="1.27.0"

$(go env GOPATH)/bin/golangci-lint --version 2>/dev/null | grep -q "version $VERSION"

if [ $? != 0 ]; then
  curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin v$VERSION
fi

GOOS=$(echo $* | cut -f1 -d-) GOARCH=$(echo $* | cut -f2 -d- | cut -f1 -d.) $(go env GOPATH)/bin/golangci-lint run
