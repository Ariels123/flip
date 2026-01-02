package commands

import (
	"fmt"

	"flip2/internal/repl"
	"flip2/internal/routing"
)

// RoutingCommand manages routing and analytics via CLI/REPL commands.
// Implements subcommands: report, dashboard, status
type RoutingCommand struct {
	apiClient *repl.APIClient
	metrics   *routing.RoutingMetrics
}

// NewRoutingCommand creates a new routing command handler.
func NewRoutingCommand(apiClient *repl.APIClient, metrics *routing.RoutingMetrics) *RoutingCommand {
	return &RoutingCommand{
		apiClient: apiClient,
		metrics:   metrics,
	}
}

// Execute implements the Command interface.
// Parses subcommand and routes to appropriate handler.
func (rc *RoutingCommand) Execute(ctx *repl.Context, args []string) error {
	if len(args) == 0 {
		return rc.handleReport()
	}

	subcommand := args[0]
	switch subcommand {
	case "report":
		return rc.handleReport()
	case "dashboard":
		return rc.handleDashboard()
	case "status":
		return rc.handleStatus()
	case "help":
		return rc.handleHelp()
	default:
		return fmt.Errorf("unknown routing subcommand: %s", subcommand)
	}
}

// handleReport generates and displays the analytics report.
func (rc *RoutingCommand) handleReport() error {
	if rc.metrics == nil {
		return fmt.Errorf("routing metrics not initialized")
	}

	report := rc.metrics.GenerateDashboard()
	fmt.Println(report)
	return nil
}

// handleDashboard generates and displays the ASCII dashboard.
func (rc *RoutingCommand) handleDashboard() error {
	if rc.metrics == nil {
		return fmt.Errorf("routing metrics not initialized")
	}

	dashboard := rc.metrics.GenerateDashboardASCII()
	fmt.Println(dashboard)
	return nil
}

// handleStatus returns current routing metrics status.
func (rc *RoutingCommand) handleStatus() error {
	if rc.metrics == nil {
		return fmt.Errorf("routing metrics not initialized")
	}

	totalTasks := rc.metrics.GetTotalTasksExecuted()
	totalCost := rc.metrics.GetTotalCost()

	fmt.Printf("Routing Status:\n")
	fmt.Printf("  Total Tasks: %d\n", totalTasks)
	fmt.Printf("  Total Cost:  $%.4f USD\n", totalCost)

	if totalTasks > 0 {
		avgCost := totalCost / float64(totalTasks)
		fmt.Printf("  Avg Cost:    $%.6f per task\n", avgCost)
	}

	return nil
}

// handleHelp displays command help information.
func (rc *RoutingCommand) handleHelp() error {
	help := `
Routing Commands:

  flip2 routing report      - Show analytics dashboard (markdown format)
  flip2 routing dashboard   - Show analytics dashboard (ASCII format)
  flip2 routing status      - Show current routing metrics
  flip2 routing help        - Show this help message

The routing system tracks task execution across different AI models
(Opus, Sonnet, Haiku, Gemini) and provides analytics on:
  - Cost breakdown by model
  - Task distribution by type
  - Savings vs always using Opus
  - Average complexity and duration by task type
`
	fmt.Println(help)
	return nil
}

// Description returns a description of the routing command.
func (rc *RoutingCommand) Description() string {
	return "Manage routing analytics and dashboards"
}
