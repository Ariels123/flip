#!/bin/bash
# Run E2E Tests for MCP Package
# This script provides convenient ways to run the comprehensive E2E test suite

set -e

PACKAGE="./internal/mcp"
TEST_PATTERN="TestE2E"

echo "MCP E2E Test Suite"
echo "=================="
echo ""

# Display usage
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  all        Run all E2E tests (default)"
    echo "  quick      Run all tests quickly"
    echo "  verbose    Run all tests with verbose output"
    echo "  coverage   Run tests and generate coverage report"
    echo "  race       Run tests with race detection"
    echo "  benchmark  Run tests with benchmarking"
    echo "  specific   Run a specific test (specify TEST_NAME env var)"
    echo "  list       List all available E2E tests"
    echo "  help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run all E2E tests"
    echo "  $0 verbose            # Run with verbose output"
    echo "  $0 coverage           # Run and show coverage"
    echo "  TEST_NAME=Concur $0 specific  # Run TestE2EConcurrentOperations"
    echo ""
    exit 0
fi

# List available tests
if [ "$1" = "list" ]; then
    echo "Available E2E Tests:"
    echo ""
    grep "^func TestE2E" internal/mcp/e2e_test.go | sed 's/func \(TestE2E[^(]*\).*/  - \1/' | sort
    echo ""
    exit 0
fi

# Run all tests (default)
if [ -z "$1" ] || [ "$1" = "all" ]; then
    echo "Running all E2E tests..."
    go test -v "$PACKAGE" -run "$TEST_PATTERN" -count=1
    echo ""
    echo "All E2E tests completed successfully!"
    exit 0
fi

# Quick run
if [ "$1" = "quick" ]; then
    echo "Running E2E tests (quick mode)..."
    go test "$PACKAGE" -run "$TEST_PATTERN" -count=1 -q
    echo ""
    echo "Quick test run completed!"
    exit 0
fi

# Verbose
if [ "$1" = "verbose" ]; then
    echo "Running E2E tests (verbose)..."
    go test -v "$PACKAGE" -run "$TEST_PATTERN" -count=1
    exit 0
fi

# Coverage
if [ "$1" = "coverage" ]; then
    echo "Running E2E tests with coverage..."
    go test -v "$PACKAGE" -run "$TEST_PATTERN" -count=1 -cover -coverprofile=coverage.out
    echo ""
    echo "Coverage report:"
    go tool cover -func=coverage.out | grep "total" || true
    echo ""
    echo "To view detailed coverage:"
    echo "  go tool cover -html=coverage.out"
    exit 0
fi

# Race detection
if [ "$1" = "race" ]; then
    echo "Running E2E tests with race detection..."
    go test -v "$PACKAGE" -run "$TEST_PATTERN" -count=1 -race
    echo ""
    echo "Race detection completed!"
    exit 0
fi

# Benchmark
if [ "$1" = "benchmark" ]; then
    echo "Running E2E tests with benchmarking..."
    go test -v "$PACKAGE" -run "$TEST_PATTERN" -count=1 -bench=. -benchmem
    exit 0
fi

# Specific test
if [ "$1" = "specific" ]; then
    if [ -z "$TEST_NAME" ]; then
        echo "Error: TEST_NAME environment variable not set"
        echo "Usage: TEST_NAME=<test_name> $0 specific"
        echo ""
        echo "Available tests:"
        grep "^func TestE2E" internal/mcp/e2e_test.go | sed 's/func \(TestE2E[^(]*\).*/  - \1/' | sort
        exit 1
    fi

    echo "Running specific test: TestE2E$TEST_NAME"
    go test -v "$PACKAGE" -run "TestE2E$TEST_NAME" -count=1
    exit 0
fi

echo "Unknown command: $1"
echo "Run '$0 --help' for usage information"
exit 1
