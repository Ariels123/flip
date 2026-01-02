package repl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

// statusFetchAPI makes an HTTP GET request to the API
func statusFetchAPI(client *APIClient, path string) (map[string]interface{}, error) {
	if client == nil {
		return nil, fmt.Errorf("api client not configured")
	}

	url := client.BaseURL() + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	if client.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+client.apiKey)
	}
	if client.authToken != "" {
		req.Header.Set("X-Auth-Token", client.authToken)
	}

	// Make request
	resp, err := client.httpClient.Do(req)
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

// statusFormatOutput formats status data for human-readable display
func statusFormatOutput(w io.Writer, status map[string]interface{}) {
	fmt.Fprintf(w, "\n╔════════════════════════════════════════════════════════════╗\n")
	fmt.Fprintf(w, "║                    FLIP2 System Status                      ║\n")
	fmt.Fprintf(w, "╚════════════════════════════════════════════════════════════╝\n\n")

	// System info
	if system, ok := status["system"].(map[string]interface{}); ok {
		fmt.Fprintf(w, "System Resources:\n")
		if gr, ok := system["goroutines"].(float64); ok {
			fmt.Fprintf(w, "  Goroutines: %v\n", int(gr))
		}
		if mem, ok := system["memory_mb"].(float64); ok {
			fmt.Fprintf(w, "  Memory (Current): %.2f MB\n", mem)
		}
		if memTotal, ok := system["memory_total_mb"].(float64); ok {
			fmt.Fprintf(w, "  Memory (Total Allocated): %.2f MB\n", memTotal)
		}
		fmt.Fprintf(w, "\n")
	}

	// Health status
	if health, ok := status["health"].(map[string]interface{}); ok {
		if hStatus, ok := health["status"].(string); ok {
			fmt.Fprintf(w, "API Health: %s\n", hStatus)
		}
		fmt.Fprintf(w, "\n")
	}

	// Agent status
	if agents, ok := status["agents"].(map[string]interface{}); ok {
		if total, ok := agents["total"].(float64); ok {
			fmt.Fprintf(w, "Active Agents: %v total\n", int(total))
		}
		if counts, ok := agents["counts"].(map[string]interface{}); ok {
			if online, ok := counts["online"].(float64); ok {
				fmt.Fprintf(w, "  Online: %v\n", int(online))
			}
			if busy, ok := counts["busy"].(float64); ok {
				fmt.Fprintf(w, "  Busy: %v\n", int(busy))
			}
			if offline, ok := counts["offline"].(float64); ok {
				fmt.Fprintf(w, "  Offline: %v\n", int(offline))
			}
		}
		fmt.Fprintf(w, "\n")
	}

	// Task status
	if tasks, ok := status["tasks"].(map[string]interface{}); ok {
		if total, ok := tasks["total"].(float64); ok {
			fmt.Fprintf(w, "Task Queue: %v total\n", int(total))
		}
		if counts, ok := tasks["counts"].(map[string]interface{}); ok {
			if pending, ok := counts["pending"].(float64); ok {
				fmt.Fprintf(w, "  Pending: %v\n", int(pending))
			}
			if running, ok := counts["running"].(float64); ok {
				fmt.Fprintf(w, "  Running: %v\n", int(running))
			}
			if completed, ok := counts["completed"].(float64); ok {
				fmt.Fprintf(w, "  Completed: %v\n", int(completed))
			}
			if failed, ok := counts["failed"].(float64); ok {
				fmt.Fprintf(w, "  Failed: %v\n", int(failed))
			}
		}
		fmt.Fprintf(w, "\n")
	}
}

// Implement StatusCommand.Execute
func (c *StatusCommand) executeStatus(ctx *Context, args []string) error {
	// Check for --json flag
	asJSON := false
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
			break
		}
	}

	// Fetch health status
	healthResp, healthErr := statusFetchAPI(ctx.APIClient, "/api/flip/health")

	// Fetch agents
	agentsResp, agentsErr := statusFetchAPI(ctx.APIClient, "/api/agents")

	// Fetch tasks
	tasksResp, tasksErr := statusFetchAPI(ctx.APIClient, "/api/tasks")

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
	}

	// Add health info
	if healthErr == nil {
		status["health"] = healthResp
	} else {
		status["health"] = map[string]string{"error": healthErr.Error()}
	}

	// Count agents by status
	agentCounts := map[string]int{
		"online":  0,
		"offline": 0,
		"busy":    0,
	}
	agentTotal := 0

	if agentsErr == nil {
		if agentsData, ok := agentsResp["agents"]; ok {
			if agents, ok := agentsData.([]interface{}); ok {
				agentTotal = len(agents)
				for _, agent := range agents {
					if a, ok := agent.(map[string]interface{}); ok {
						if agentStatus, ok := a["status"].(string); ok {
							if count, exists := agentCounts[agentStatus]; exists {
								agentCounts[agentStatus] = count + 1
							}
						}
					}
				}
			}
		}
	}
	status["agents"] = map[string]interface{}{
		"total":  agentTotal,
		"counts": agentCounts,
		"error":  agentsErr,
	}

	// Count tasks by status
	taskCounts := map[string]int{
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
	}
	taskTotal := 0

	if tasksErr == nil {
		if tasksData, ok := tasksResp["tasks"]; ok {
			if tasks, ok := tasksData.([]interface{}); ok {
				taskTotal = len(tasks)
				for _, task := range tasks {
					if t, ok := task.(map[string]interface{}); ok {
						if taskStatus, ok := t["status"].(string); ok {
							if count, exists := taskCounts[taskStatus]; exists {
								taskCounts[taskStatus] = count + 1
							}
						}
					}
				}
			}
		}
	}
	status["tasks"] = map[string]interface{}{
		"total":  taskTotal,
		"counts": taskCounts,
		"error":  tasksErr,
	}

	// Output result
	if asJSON {
		// Output as JSON
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal status: %w", err)
		}
		fmt.Fprintf(ctx.Output, "%s\n", data)
	} else {
		// Output as human-readable format
		statusFormatOutput(ctx.Output, status)
	}

	return nil
}
