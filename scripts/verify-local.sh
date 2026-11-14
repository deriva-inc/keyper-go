#!/usr/bin/env bash
set -euo pipefail

echo "======================================================="
echo "   Keyper Local Pre-flight Verification & Cleanup      "
echo "======================================================="

# Trap to guarantee cleanup runs even if tests or builds fail
cleanup() {
    echo ""
    echo "--> Cleaning up local build artifacts & temporary files..."
    rm -rf bin/ tmp/ coverage.out coverage.html *.test
    echo "--> All artifacts removed. Local disk space reclaimed."
}
trap cleanup EXIT

echo "1/5: Verifying Go module dependencies..."
go mod verify

echo "2/5: Running static analysis (go vet)..."
go vet ./...

echo "3/5: Running vulnerability scanner (govulncheck)..."
govulncheck ./...

echo "4/5: Running test suite with coverage..."
go test -v -coverprofile=coverage.out -covermode=atomic ./...

echo "5/5: Verifying static binary compilation (linux/amd64 & linux/arm64)..."
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/keyper-test-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/keyper-test-arm64 .

echo ""
echo "======================================================="
echo "   ALL LOCAL CHECKS PASSED SUCCESSFULLY!               "
echo "======================================================="
