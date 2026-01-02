package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"
)

// StatusCommand implements the /status command for showing system status.
// Displays daemon status, active agents, running tasks, and resource metrics.
type StatusCommand struct {
	apiClient APIClientInterface
}

// NewStatusCommand creates a new status command handler.
func NewStatusCommand(apiClient APIClientInterface) *StatusCommand {
	return &StatusCommand{
		apiClient: apiClient,
	}
}

// Name returns the command name.
func (c *StatusCommand) Name() string {
	return "status"
}

// Aliases returns alternative command names.
func (c *StatusCommand) Aliases() []string {
	return []string{"s", "st"}
}

// Description returns a short description of the command.
func (c *StatusCommand) Description() string {
	return "Show system status"
}

// Usage returns the usage string.
func (c *StatusCommand) Usage() string {
	return "/status [--json]"
}

// Help returns detailed help text.
func (c *StatusCommand) Help() string {
	return `Show FLIP2 system status.

Usage: /status [--json]

Options:
  --json    Output in JSON format

Examples:
  /status           Show human-readable status
  /status --json    Show machine-readable status

Output includes:
  - System resources (goroutines, memory usage)
  - API health status
  - Agent summary (online, offline, busy counts)
  - Task queue statistics (pending, running, completed, failed)
`
}

// Execute implements the Command interface.
func (c *StatusCommand) Execute(ctx interface{}, args []string) error {
	// For now, return a mock implementation that displays system resources
	// In a real implementation, this would call API endpoints

	// Check for --json flag
	asJSON := false
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
			break
		}
	}

	// Get resource metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	goroutines := runtime.NumGoroutine()

	// Build status data
	status := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"system": map[string]interface{}{
			"goroutines":       goroutines,
			"memory_mb":        float64(m.Alloc) / 1024 / 1024,
			"memory_total_mb":  float64(m.TotalAlloc) / 1024 / 1024,
		},
		"health": map[string]string{
			"status": "healthy",
		},
		"agents": map[string]interface{}{
			"total": 0,
			"counts": map[string]int{
				"online":  0,
				"offline": 0,
				"busy":    0,
			},
		},
		"tasks": map[string]interface{}{
			"total": 0,
			"counts": map[string]int{
				"pending":   0,
				"running":   0,
				"completed": 0,
				"failed":    0,
			},
		},
	}

	// Output result
	if asJSON {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal status: %w", err)
		}
		// Try to write to output if available
		if output, ok := ctx.(io.Writer); ok {
			fmt.Fprintf(output, "%s\n", data)
		} else {
			fmt.Printf("%s\n", data)
		}
	} else {
		// Output as human-readable format
		c.formatOutput(ctx, status)
	}

	return nil
}

// Complete provides tab completion suggestions.
func (c *StatusCommand) Complete(ctx interface{}, args []string, pos int, partial string) []interface{} {
	if pos == 0 && strings.HasPrefix("--json", partial) {
		return []interface{}{
			map[string]interface{}{
				"value":       "--json",
				"description": "Output as JSON",
			},
		}
	}
	return nil
}

// formatOutput formats status data for human-readable display
func (c *StatusCommand) formatOutput(ctx interface{}, status map[string]interface{}) {
	// Get output writer if available
	var w io.Writer = nil
	if output, ok := ctx.(io.Writer); ok {
		w = output
	}

	// Helper function to write
	write := func(format string, args ...interface{}) {
		if w != nil {
			fmt.Fprintf(w, format, args...)
		} else {
			fmt.Printf(format, args...)
		}
	}

	write("\n")
	write("╔════════════════════════════════════════════════════════════╗\n")
	write("║                    FLIP2 System Status                      ║\n")
	write("╚════════════════════════════════════════════════════════════╝\n\n")

	// System info
	if system, ok := status["system"].(map[string]interface{}); ok {
		write("System Resources:\n")
		if gr, ok := system["goroutines"].(float64); ok {
			write("  Goroutines:        %v\n", int(gr))
		}
		if mem, ok := system["memory_mb"].(float64); ok {
			write("  Memory (Current):  %.2f MB\n", mem)
		}
		if memTotal, ok := system["memory_total_mb"].(float64); ok {
			write("  Memory (Total):    %.2f MB\n", memTotal)
		}
		write("\n")
	}

	// Health status
	if health, ok := status["health"].(map[string]interface{}); ok {
		if hStatus, ok := health["status"].(string); ok {
			write("API Health:        %s\n", hStatus)
		}
		write("\n")
	}

	// Agent status table
	if agents, ok := status["agents"].(map[string]interface{}); ok {
		if total, ok := agents["total"].(float64); ok {
			write("Active Agents:     %v total\n", int(total))
		}
		if counts, ok := agents["counts"].(map[string]interface{}); ok {
			if w != nil {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				write("  Status\tCount\n")
				write("  ------\t-----\n")

				if online, ok := counts["online"].(float64); ok {
					fmt.Fprintf(tw, "  Online\t%v\n", int(online))
				}
				if busy, ok := counts["busy"].(float64); ok {
					fmt.Fprintf(tw, "  Busy\t%v\n", int(busy))
				}
				if offline, ok := counts["offline"].(float64); ok {
					fmt.Fprintf(tw, "  Offline\t%v\n", int(offline))
				}
				tw.Flush()
			}
		}
		write("\n")
	}

	// Task status table
	if tasks, ok := status["tasks"].(map[string]interface{}); ok {
		if total, ok := tasks["total"].(float64); ok {
			write("Task Queue:        %v total\n", int(total))
		}
		if counts, ok := tasks["counts"].(map[string]interface{}); ok {
			if w != nil {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				write("  Status\tCount\n")
				write("  ------\t-----\n")

				if pending, ok := counts["pending"].(float64); ok {
					fmt.Fprintf(tw, "  Pending\t%v\n", int(pending))
				}
				if running, ok := counts["running"].(float64); ok {
					fmt.Fprintf(tw, "  Running\t%v\n", int(running))
				}
				if completed, ok := counts["completed"].(float64); ok {
					fmt.Fprintf(tw, "  Completed\t%v\n", int(completed))
				}
				if failed, ok := counts["failed"].(float64); ok {
					fmt.Fprintf(tw, "  Failed\t%v\n", int(failed))
				}
				tw.Flush()
			}
		}
		write("\n")
	}
}

// fetchAPI makes an HTTP GET request to the API
func (c *StatusCommand) fetchAPI(path string) (map[string]interface{}, error) {
	if c.apiClient == nil {
		return nil, fmt.Errorf("api client not configured")
	}

	url := c.apiClient.BaseURL() + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")

	// Make request with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if errMsg, ok := result["error"].(string); ok {
			return nil, fmt.Errorf("api error: %s", errMsg)
		}
		return nil, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	return result, nil
}
