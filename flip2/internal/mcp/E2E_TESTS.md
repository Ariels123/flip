# MCP End-to-End Tests

## Overview

The `e2e_test.go` file contains comprehensive end-to-end tests for the Model Context Protocol (MCP) server integration. These tests verify the complete MCP lifecycle without requiring a real subprocess server.

## Test Coverage

### 1. Connection Lifecycle (`TestE2EConnectionLifecycle`)

Tests the complete server connection lifecycle:
- Server initialization with client handshake
- Protocol version negotiation
- Server capabilities discovery
- Ping/keep-alive functionality
- Graceful connection closure
- Verification that connection is properly closed

**Key assertions:**
- Protocol version matches expected (2025-06-18)
- Server capabilities are returned correctly
- Ping responds without error
- Server is properly closed

### 2. Tool Discovery (`TestE2EToolDiscovery`)

Tests discovering available tools from a server:
- List all tools exposed by a server
- Parse tool metadata (name, description, input schema)
- Verify tool annotations (readOnly, destructive, etc.)
- Handle multiple tools from single server
- Preserve schema information

**Key assertions:**
- Correct number of tools discovered
- Tool names and descriptions match
- Annotations are preserved correctly
- ReadOnlyHint and DestructiveHint flags are set appropriately

### 3. Tool Invocation (`TestE2EToolInvocation`)

Tests calling tools with arguments and receiving results:
- Invoke tools with map-based arguments
- Parse and execute tool-specific logic
- Return properly formatted results
- Handle tool not found errors
- Support multiple data types (numbers, strings, etc.)

**Key assertions:**
- Tool result is returned correctly
- Result content type is valid
- Computed results are accurate
- Non-existent tools return errors
- Error messages are descriptive

### 4. Resource Listing (`TestE2EResourceListing`)

Tests discovering resources from a server:
- List all resources exposed by a server
- Parse resource metadata (URI, name, MIME type)
- Handle resource annotations and priorities
- Support resource descriptions

**Key assertions:**
- Correct number of resources discovered
- Resource URIs are properly formatted
- Annotations include audience and priority
- MIME types are set correctly

### 5. Resource Reading (`TestE2EResourceReading`)

Tests reading resource content:
- Retrieve content by resource URI
- Support different content types (JSON, text, binary)
- Handle missing resources with proper errors
- Verify error codes (ResourceNotFound)

**Key assertions:**
- Resource content is returned accurately
- URI in response matches request
- Non-existent resources return 404 error
- Error codes are correct (ErrorCodeResourceNotFound)

### 6. Prompt Listing (`TestE2EPromptListing`)

Tests discovering prompt templates:
- List all prompts from a server
- Parse prompt metadata (name, description)
- Handle prompt arguments with required flags
- Support argument descriptions

**Key assertions:**
- Correct number of prompts discovered
- Prompt names are correct
- Argument lists are complete
- Required flags are set appropriately

### 7. Prompt Execution (`TestE2EPromptExecution`)

Tests executing prompt templates with arguments:
- Execute prompts with provided arguments
- Return formatted conversation
- Handle user and assistant roles
- Support message content

**Key assertions:**
- Prompt execution returns messages
- Message roles are correct (user/assistant)
- Content type is text
- Messages include actual arguments passed

### 8. Sampling Requests (`TestE2ESamplingRequest`)

Tests servers requesting LLM completions:
- Handle sampling requests from servers
- Return completion with model and stop reason
- Support message history
- Handle different context inclusion modes

**Key assertions:**
- Response role is "assistant"
- Content type is correct
- Model name is preserved
- Stop reason indicates completion

### 9. Registry Management (`TestE2ERegistryAndDiscovery`)

Tests managing multiple servers:
- Register multiple servers
- List all registered servers
- Get specific servers by name
- Filter servers by capability
- Deregister servers properly

**Key assertions:**
- All servers are registered and discoverable
- Server retrieval returns correct servers
- Capability filtering works correctly
- Deregistration removes servers

### 10. Error Recovery (`TestE2EErrorRecovery`)

Tests handling transient failures:
- Simulate transient errors
- Verify retry behavior
- Confirm recovery after failures
- Track failure counts

**Key assertions:**
- Failures are detected correctly
- System recovers on subsequent attempts
- Failure counts are accurate
- Recovered operations return correct results

### 11. Concurrent Operations (`TestE2EConcurrentOperations`)

Tests concurrent tool invocation:
- Handle multiple simultaneous requests
- Maintain thread safety
- Verify call counting accuracy
- Support goroutine-based parallelism

**Key assertions:**
- All concurrent calls succeed
- No race conditions detected
- Call count matches expected concurrency level
- Results are consistent

### 12. Timeout Handling (`TestE2ETimeoutHandling`)

Tests respecting context timeouts:
- Enforce request timeouts
- Detect deadline exceeded errors
- Support long-running operations
- Cancel operations on timeout

**Key assertions:**
- Timeout is respected
- DeadlineExceeded error is returned
- Operations complete within timeout window

## Running the Tests

### Run all E2E tests:
```bash
cd /Users/arielspivakovsky/src/flip/flip2
go test -v ./internal/mcp -run "TestE2E" -count=1
```

### Run a specific test:
```bash
go test -v ./internal/mcp -run "TestE2EToolDiscovery" -count=1
```

### Run with coverage:
```bash
go test -v ./internal/mcp -run "TestE2E" -cover -count=1
```

### Run with race detection:
```bash
go test -v ./internal/mcp -run "TestE2E" -race -count=1
```

## Test Structure

Each test follows a consistent pattern:

1. **Setup**: Create mock servers with specific configurations
2. **Execute**: Call MCP methods and operations
3. **Verify**: Assert results match expectations
4. **Cleanup**: Ensure resources are properly closed

## Mock Servers

The tests use several mock server implementations that extend the base `mockServer`:

### mockServerWithBehavior
Implements realistic tool execution logic with:
- Calculator operations (add, multiply)
- Argument validation
- Result formatting

### mockServerWithResources
Provides resource content for testing:
- URI-to-content mapping
- ResourceNotFound errors
- JSON and text content types

### mockServerWithPrompts
Implements prompt template execution:
- Message generation
- Role assignment (user/assistant)
- Argument substitution

### mockServerWithFailures
Simulates transient failures:
- Configurable failure counts
- Automatic recovery after N attempts
- Error code generation

### mockServerWithConcurrency
Tracks concurrent access:
- Call counting with mutex protection
- Simulated work (sleep)
- Thread-safe operations

### mockServerWithDelay
Simulates slow operations:
- Configurable delay duration
- Context awareness
- Timeout handling

## Key Testing Patterns

### Testing with Contexts
All tests use `context.Background()` or `context.WithTimeout()` to respect cancellation and deadlines:

```go
ctx := context.Background()
// or
ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
defer cancel()
```

### Error Testing
Tests verify specific error types and codes:

```go
if mcpErr, ok := err.(*Error); ok {
    if mcpErr.Code != ErrorCodeResourceNotFound {
        t.Errorf("expected ResourceNotFound error, got %d", mcpErr.Code)
    }
}
```

### Concurrent Testing
Tests use goroutines with wait groups and result channels:

```go
var wg sync.WaitGroup
results := make(chan error, concurrentCalls)
for i := 0; i < concurrentCalls; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        _, err := server.CallTool(ctx, "op", args)
        results <- err
    }()
}
wg.Wait()
close(results)
```

## Expected Test Results

When all tests pass, you should see output like:

```
ok      flip2/internal/mcp      2.345s
```

If tests fail, the output will show:
- Which test failed
- What assertion failed
- Expected vs actual values
- Suggested fixes

## Troubleshooting

### Import Issues
If you see "cannot find module providing package", run:
```bash
go mod tidy
```

### Build Failures
The codebase has pre-existing build issues with duplicate type definitions in `matcher.go` and `router.go`. These are separate from the E2E tests.

To test only the E2E functionality without other build issues, the E2E tests are designed to be self-contained within `e2e_test.go`.

### Timeout Issues
If tests timeout, increase the timeout duration:
```bash
go test -timeout 30s ./internal/mcp -run "TestE2E"
```

## Future Enhancements

Potential additions to E2E test coverage:

1. **Stdio Transport Tests**: Test actual subprocess communication with JSON-RPC
2. **Real MCP Servers**: Integration tests with actual MCP server implementations
3. **Performance Tests**: Benchmark tool invocation and resource reading
4. **Stress Tests**: Test with high concurrency and large payloads
5. **Protocol Version Tests**: Verify compatibility with different protocol versions
6. **Subscription Tests**: Test resource update subscriptions
7. **Completion Tests**: Test argument auto-completion
8. **Network Resilience**: Test reconnection after network failures

## Contributing

When adding new tests:

1. Create a new test function: `func TestE2E<Feature>(t *testing.T)`
2. Follow the Setup → Execute → Verify pattern
3. Document the test with a comment explaining what it verifies
4. Add a corresponding mock server type if needed
5. Update this document with the new test coverage

## Related Files

- `/Users/arielspivakovsky/src/flip/flip2/internal/mcp/server.go` - Core MCP interfaces
- `/Users/arielspivakovsky/src/flip/flip2/internal/mcp/discovery.go` - Tool discovery
- `/Users/arielspivakovsky/src/flip/flip2/internal/mcp/invoker.go` - Tool invocation
- `/Users/arielspivakovsky/src/flip/flip2/internal/mcp/registry_test.go` - Base mock implementations
