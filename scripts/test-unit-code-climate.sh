#!/usr/bin/env sh

# Purpose: This script runs the unit tests and generate Code Climate coverage files.
# This target runs from the test image in Circle CI.
# Instructions: make test-unit-code-climate

set -eu

GO_PACKAGES=$(go list ./... | grep -v "test/")

for pkg in $GO_PACKAGES; do
    go test -coverprofile=$(echo $pkg | tr / -).cover.tmp $pkg;
done

echo "mode: set" > unit.cover
grep -h -v "^mode:" ./*.cover.tmp >> unit.cover
rm -f *.cover.tmp
