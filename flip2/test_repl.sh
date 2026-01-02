#!/bin/bash

echo "Testing REPL Implementation (SLC-002)"
echo "======================================"
echo ""

# Build binary
echo "Building flip2 binary..."
go build -o flip2_test ./cmd/flip2/main.go || {
    echo "FAILED: Could not build flip2"
    exit 1
}
echo "OK: Binary built"
echo ""

# Test 1: Prompt appears
echo "Test 1: Prompt appears on startup"
output=$(echo "/exit" | ./flip2_test 2>&1)
if echo "$output" | grep -q "FLIP2 Multi-Agent Coordination Shell"; then
    echo "OK: REPL banner appears"
else
    echo "FAILED: Banner not found"
    exit 1
fi
echo ""

# Test 2: Accepts input
echo "Test 2: Accepts user input"
output=$(echo -e "/help" | ./flip2_test 2>&1)
if echo "$output" | grep -q "Available commands:"; then
    echo "OK: Input accepted and processed"
else
    echo "FAILED: Input not processed"
    exit 1
fi
echo ""

# Test 3: Exit cleanly
echo "Test 3: Exits cleanly"
if echo "/exit" | ./flip2_test 2>&1 > /dev/null; then
    echo "OK: Exited with code 0"
else
    echo "FAILED: Exit code non-zero"
    exit 1
fi
echo ""

# Test 4: Help command works
echo "Test 4: Help command displays commands"
output=$(echo -e "/help" | ./flip2_test 2>&1)
if echo "$output" | grep -q "/status" && echo "$output" | grep -q "/send"; then
    echo "OK: Help displays status and send commands"
else
    echo "FAILED: Help doesn't show expected commands"
    exit 1
fi
echo ""

# Test 5: Help for specific command
echo "Test 5: Help for specific command"
output=$(echo -e "/help send" | ./flip2_test 2>&1)
if echo "$output" | grep -q "Send a signal to an agent"; then
    echo "OK: Command-specific help works"
else
    echo "FAILED: Command help not found"
    exit 1
fi
echo ""

# Test 6: Error handling for unknown commands
echo "Test 6: Error handling for unknown commands"
output=$(echo -e "/unknowncommand" | ./flip2_test 2>&1)
if echo "$output" | grep -q "unknown command"; then
    echo "OK: Error handling works"
else
    echo "FAILED: Error handling not working"
    exit 1
fi
echo ""

# Test 7: Aliases work
echo "Test 7: Command aliases work"
output=$(echo -e "/h" | ./flip2_test 2>&1)
if echo "$output" | grep -q "Available commands:"; then
    echo "OK: Alias '/h' works for '/help'"
else
    echo "FAILED: Alias not working"
    exit 1
fi
echo ""

# Test 8: Quit aliases
echo "Test 8: Multiple exit aliases"
if echo -e "/q" | ./flip2_test 2>&1 > /dev/null; then
    echo "OK: '/q' alias for exit works"
else
    echo "FAILED: Exit alias failed"
    exit 1
fi
echo ""

# Test 9: Banner displays on startup
echo "Test 9: Banner displays"
output=$(echo "/exit" | ./flip2_test 2>&1)
if echo "$output" | grep -q "FLIP2 Multi-Agent Coordination Shell"; then
    echo "OK: Banner displays correctly"
else
    echo "FAILED: Banner not found"
    exit 1
fi
echo ""

# Test 10: Interactive flag works
echo "Test 10: --interactive flag"
if echo "/exit" | ./flip2_test --interactive 2>&1 | grep -q "FLIP2 Multi-Agent"; then
    echo "OK: --interactive flag works"
else
    echo "FAILED: --interactive flag not working"
    exit 1
fi
echo ""

echo "======================================"
echo "All acceptance tests PASSED!"
echo "======================================"
