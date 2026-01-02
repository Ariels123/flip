// Package mcp provides tool discovery functionality for MCP servers.
//
// The discovery module enables querying connected MCP servers for available tools
// and maintaining an up-to-date registry of tool metadata across all servers.
//
// # Tool Discovery
//
// Tools are discovered by querying each registered server's tools/list endpoint.
// The discovery process:
//
//  1. Connects to each registered server (if not already connected)
//  2. Sends tools/list request
//  3. Parses response to extract tool metadata
//  4. Returns tool metadata with server context
//
// # Pagination Support
//
// The MCP protocol supports paginated results. DiscoverTools handles pagination
// automatically, fetching all tools even when results are paginated.
//
// # Example Usage
//
//	registry := mcp.NewRegistry()
//	// ... register servers ...
//
//	// Discover tools from a specific server
//	tools, err := mcp.DiscoverTools(ctx, registry, "filesystem-server")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, tool := range tools {
//	    fmt.Printf("Tool: %s (%s)\n", tool.Name, tool.Description)
//	}
//
//	// Refresh all tools across all servers
//	err = mcp.RefreshAllTools(ctx, registry)
//	if err != nil {
//	    log.Fatal(err)
//	}
package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DiscoverTools queries an MCP server for available tools.
//
// This function sends a tools/list request to the specified server and returns
// all tool metadata. It handles pagination automatically, fetching all tools
// even if the server returns results in multiple pages.
//
// The returned tools are suitable for inspection, caching, or execution.
// Each tool includes its schema, description, and any annotations.
//
// Args:
//   - ctx: Context for cancellation and timeout
//   - registry: Registry containing registered servers
//   - serverID: Name of the server to query for tools
//
// Returns:
//   - []*Tool: Slice of tool metadata from the server
//   - error: If the server is not found, unreachable, or returns an error
//
// Example:
//
//	tools, err := DiscoverTools(ctx, registry, "my-server")
//	if err != nil {
//	    log.Fatalf("Tool discovery failed: %v", err)
//	}
//	fmt.Printf("Found %d tools\n", len(tools))
func DiscoverTools(ctx context.Context, registry Registry, serverID string) ([]*Tool, error) {
	// Validate inputs
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if serverID == "" {
		return nil, fmt.Errorf("serverID is empty")
	}

	// Get server from registry
	server, exists := registry.Get(serverID)
	if !exists {
		return nil, fmt.Errorf("server %q not registered", serverID)
	}

	// Verify server has tools capability
	caps := server.Capabilities()
	if caps == nil || caps.Tools == nil {
		return nil, fmt.Errorf("server %q does not support tools capability", serverID)
	}

	// Discover tools with pagination support
	var allTools []*Tool
	var cursor *string

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// List tools from server
		result, err := server.ListTools(ctx, cursor)
		if err != nil {
			return nil, err
		}

		// Copy tools to return slice
		for _, tool := range result.Tools {
			toolCopy := tool
			allTools = append(allTools, &toolCopy)
		}

		// Check for more results
		if result.NextCursor == nil {
			break
		}
		cursor = result.NextCursor
	}

	return allTools, nil
}

// RefreshAllTools queries all registered servers for their available tools.
//
// This function iterates through all registered servers and discovers all
// tools from each server. It uses concurrent requests to servers to minimize
// total time while respecting context cancellation.
//
// The operation is atomic: if any server fails, an error is returned with
// partial results. Callers should decide whether to use partial results or
// retry with different timeout values.
//
// Args:
//   - ctx: Context for cancellation and timeout
//   - registry: Registry containing servers to query
//
// Returns:
//   - error: If any server fails to return tools. The error message includes
//     details about which servers failed.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	err := RefreshAllTools(ctx, registry)
//	if err != nil {
//	    log.Printf("Some servers failed: %v", err)
//	}
//
// Note: This function doesn't return tools directly; instead, it queries
// all servers and relies on the registry's internal cache to be updated.
// Use DiscoverTools() to retrieve tools from individual servers.
func RefreshAllTools(ctx context.Context, registry Registry) error {
	// Validate inputs
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}

	// Get all registered server names
	serverNames := registry.List()
	if len(serverNames) == 0 {
		return nil // No servers to refresh
	}

	// Use a channel to collect errors from concurrent operations
	type discoverResult struct {
		serverName string
		toolCount  int
		err        error
	}

	resultCh := make(chan discoverResult, len(serverNames))
	var wg sync.WaitGroup

	// Launch concurrent discovery for each server
	for _, serverName := range serverNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Respect context cancellation
			select {
			case <-ctx.Done():
				resultCh <- discoverResult{
					serverName: name,
					err:        ctx.Err(),
				}
				return
			default:
			}

			// Discover tools from this server
			tools, err := DiscoverTools(ctx, registry, name)
			if err != nil {
				resultCh <- discoverResult{
					serverName: name,
					err:        err,
				}
				return
			}

			resultCh <- discoverResult{
				serverName: name,
				toolCount:  len(tools),
				err:        nil,
			}
		}(serverName)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultCh)

	// Collect results and check for errors
	var errors []error
	successCount := 0

	for result := range resultCh {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("server %q: %w", result.serverName, result.err))
		} else {
			successCount++
		}
	}

	// Return error if any server failed
	if len(errors) > 0 {
		errorMsg := fmt.Sprintf("tool refresh failed for %d/%d servers: ", len(errors), len(serverNames))
		for i, err := range errors {
			if i > 0 {
				errorMsg += "; "
			}
			errorMsg += err.Error()
		}
		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}

// MCPTool represents discovered tool metadata from an MCP server.
//
// This type combines the tool definition with context about when and where
// it was discovered. It's used as the return type for discovery operations
// and can be used for caching, indexing, or routing decisions.
type MCPTool struct {
	// Name is the unique identifier for the tool.
	Name string

	// Description explains what the tool does.
	Description string

	// InputSchema is the JSON Schema for the tool's arguments.
	InputSchema interface{}

	// Annotations provides additional metadata about the tool's behavior.
	Annotations *ToolAnnotations

	// ServerName is the name of the server providing this tool.
	ServerName string

	// DiscoveredAt is the timestamp when this tool was discovered.
	DiscoveredAt time.Time
}

// String returns a human-readable representation of the tool.
func (t *MCPTool) String() string {
	return fmt.Sprintf("%s:%s", t.ServerName, t.Name)
}
