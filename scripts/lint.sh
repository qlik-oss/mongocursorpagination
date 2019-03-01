#!/usr/bin/env sh

# Purpose: This script lints the code.
# Instructions: make lint

set -eu

go get -u github.com/alecthomas/gometalinter
gometalinter --install
gometalinter --config=gometalinter.json ./
