package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"flip2/internal/budget"
	"github.com/spf13/cobra"
)

func budgetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Manage and track resource budgets",
		Long:  "Configure, track, and report on spending budgets for agents, models, and global usage.",
	}

	cmd.AddCommand(budgetListCmd())
	cmd.AddCommand(budgetGetCmd())
	cmd.AddCommand(budgetReportCmd())
	cmd.AddCommand(budgetResetCmd())
	cmd.AddCommand(budgetCreateCmd())
	cmd.AddCommand(budgetUpdateCmd())

	return cmd
}

func budgetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured budgets",
		Run: func(cmd *cobra.Command, args []string) {
			req, _ := http.NewRequest("GET", apiURL+"/api/budgets", nil)
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Failed to list budgets", "error", err)
				return
			}
			defer resp.Body.Close()

			var result struct {
				Budgets []*budget.Budget `json:"budgets"`
				Count   int              `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Error("Failed to decode response", "error", err)
				return
			}

			if result.Count == 0 {
				fmt.Println("No budgets configured.")
				return
			}

			fmt.Printf("%-20s %-10s %-15s %-10s %-10s %-10s %-10s\n", 
				"NAME", "SCOPE", "SCOPE_ID", "LIMIT", "CONSUMED", "STATUS", "ENABLED")
			fmt.Println(strings.Repeat("-", 85))

			for _, b := range result.Budgets {
				statusColor := ""
				if b.Status == budget.StatusExhausted {
					statusColor = "\033[31m" // Red
				} else if b.Status == budget.StatusWarning {
					statusColor = "\033[33m" // Yellow
				}
				reset := "\033[0m"

				fmt.Printf("%-20s %-10s %-15s %-10.2f %-10.4f %s%-10s%s %-10v\n",
					truncate(b.Name, 20),
					b.Scope,
					b.ScopeID,
					b.LimitUSD,
					b.ConsumedUSD,
					statusColor, b.Status, reset,
					b.Enabled,
				)
			}
		},
	}
}

func budgetGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get detailed budget information",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			req, _ := http.NewRequest("GET", apiURL+"/api/budgets/"+id, nil)
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Failed to get budget", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("Budget %s not found.\n", id)
				return
			}

			var b budget.Budget
			if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
				logger.Error("Failed to decode response", "error", err)
				return
			}

			fmt.Printf("Budget: %s (%s)\n", b.Name, b.ID)
			fmt.Println(strings.Repeat("=", 40))
			fmt.Printf("Scope:        %s\n", b.Scope)
			fmt.Printf("Scope ID:     %s\n", b.ScopeID)
			fmt.Printf("Limit:        $%.2f USD\n", b.LimitUSD)
			fmt.Printf("Consumed:     $%.4f USD (%.1f%%)\n", b.ConsumedUSD, (b.ConsumedUSD/b.LimitUSD)*100)
			fmt.Printf("Status:       %s\n", b.Status)
			fmt.Printf("Reset Period: %s\n", b.ResetPeriod)
			fmt.Printf("Enabled:      %v\n", b.Enabled)
			fmt.Printf("Last Reset:   %s\n", b.LastResetAt.Format(time.RFC3339))
			fmt.Printf("Next Reset:   %s\n", b.NextResetAt.Format(time.RFC3339))
		},
	}
}

func budgetReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report <id>",
		Short: "Generate a detailed budget usage report",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			req, _ := http.NewRequest("GET", apiURL+"/api/budgets/"+id+"/report", nil)
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Failed to get budget report", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("Budget %s not found.\n", id)
				return
			}

			var r budget.BudgetReport
			if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
				logger.Error("Failed to decode response", "error", err)
				return
			}

			fmt.Printf("\nBUDGET REPORT: %s\n", r.Budget.Name)
			fmt.Println(strings.Repeat("=", 60))
			fmt.Printf("Period:       %s to %s\n", r.Budget.LastResetAt.Format("Jan 02 15:04"), time.Now().Format("Jan 02 15:04"))
			fmt.Printf("Usage:        $%.4f / $%.2f (%.1f%%)\n", r.TotalConsumed, r.Budget.LimitUSD, r.ConsumedPct)
			fmt.Printf("Remaining:    $%.4f USD\n", r.RemainingUSD)
			fmt.Printf("Current Rate: $%.4f USD/hour\n", r.ConsumptionRate)
			fmt.Printf("Projected:    $%.4f USD by end of period\n", r.ProjectedUsage)
			fmt.Printf("Status:       %s\n", r.Budget.Status)

			fmt.Println("\nTOP CONSUMERS:")
			fmt.Printf("%-20s %-10s %-12s %-10s\n", "CONSUMER", "TYPE", "USD", "PERCENT")
			fmt.Println(strings.Repeat("-", 55))
			for _, c := range r.TopConsumers {
				fmt.Printf("%-20s %-10s %-12.4f %-10.1f%%\n", truncate(c.ID, 20), c.Type, c.ConsumedUSD, c.Percentage)
			}

			if len(r.RecentActivity) > 0 {
				fmt.Println("\nRECENT ACTIVITY:")
				fmt.Printf("%-15s %-15s %-15s %-10s\n", "TIME", "AGENT", "MODEL", "USD")
				fmt.Println(strings.Repeat("-", 55))
				for i := len(r.RecentActivity) - 1; i >= 0; i-- {
					a := r.RecentActivity[i]
					fmt.Printf("%-15s %-15s %-15s %-10.4f\n", 
						a.Timestamp.Format("Jan 02 15:04"),
						truncate(a.AgentID, 15),
						truncate(a.Model, 15),
						a.AmountUSD)
				}
			}
			fmt.Println()
		},
	}
}

func budgetResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset <id>",
		Short: "Manually reset a budget",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			req, _ := http.NewRequest("POST", apiURL+"/api/budgets/"+id+"/reset", nil)
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Failed to reset budget", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.Error("Reset failed", "status", resp.Status, "body", string(body))
				return
			}

			fmt.Printf("Budget %s reset successfully.\n", id)
		},
	}
}

func budgetCreateCmd() *cobra.Command {
	var name, scope, scopeID, period string
	var limit float64

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new budget",
		Run: func(cmd *cobra.Command, args []string) {
			if name == "" || scope == "" || scopeID == "" || period == "" || limit <= 0 {
				fmt.Println("Error: All flags --name, --scope, --scope-id, --limit, --period are required and limit must be > 0")
				return
			}

			reqBody := map[string]interface{}{
				"name":         name,
				"scope":        scope,
				"scope_id":     scopeID,
				"limit_usd":    limit,
				"reset_period": period,
			}

			jsonData, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", apiURL+"/api/budgets", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Failed to create budget", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				logger.Error("Creation failed", "status", resp.Status, "body", string(body))
				return
			}

			var b budget.Budget
			json.NewDecoder(resp.Body).Decode(&b)
			fmt.Printf("Budget created successfully: %s (%s)\n", b.Name, b.ID)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the budget")
	cmd.Flags().StringVar(&scope, "scope", "global", "Scope (agent, model, global)")
	cmd.Flags().StringVar(&scopeID, "scope-id", "global", "Scope ID (agent_id, model name, or 'global')")
	cmd.Flags().Float64Var(&limit, "limit", 0, "Budget limit in USD")
	cmd.Flags().StringVar(&period, "period", "daily", "Reset period (hourly, daily, weekly, monthly)")

	return cmd
}

func budgetUpdateCmd() *cobra.Command {
	var limit float64
	var enabled, disabled bool

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing budget",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			
			reqBody := make(map[string]interface{})
			if limit > 0 {
				reqBody["limit_usd"] = limit
			}
			if enabled {
				reqBody["enabled"] = true
			} else if disabled {
				reqBody["enabled"] = false
			}

			if len(reqBody) == 0 {
				fmt.Println("Error: Nothing to update. Use --limit, --enable, or --disable.")
				return
			}

			jsonData, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("PATCH", apiURL+"/api/budgets/"+id, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Failed to update budget", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.Error("Update failed", "status", resp.Status, "body", string(body))
				return
			}

			fmt.Printf("Budget %s updated successfully.\n", id)
		},
	}

	cmd.Flags().Float64Var(&limit, "limit", 0, "New budget limit in USD")
	cmd.Flags().BoolVar(&enabled, "enable", false, "Enable the budget")
	cmd.Flags().BoolVar(&disabled, "disable", false, "Disable the budget")

	return cmd
}
