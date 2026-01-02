package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func sessionCmd() *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage FLIP2 sessions",
		Long:  "Create, attach, list, and manage session lifecycles",
	}

	sessionCmd.AddCommand(sessionListCmd())
	sessionCmd.AddCommand(sessionAttachCmd())
	sessionCmd.AddCommand(sessionStartCmd())
	sessionCmd.AddCommand(sessionStopCmd())

	return sessionCmd
}

// sessionListCmd lists all available sessions
func sessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		Long:  "Display all available sessions with their status and metadata",
		Run: func(cmd *cobra.Command, args []string) {
			coordinatorID, _ := cmd.Flags().GetString("coordinator")
			status, _ := cmd.Flags().GetString("status")

			// Build query parameters
			params := url.Values{}
			if coordinatorID != "" {
				params.Add("coordinator_id", coordinatorID)
			}
			if status != "" {
				params.Add("status", status)
			}

			reqURL := apiURL + "/api/sessions/list"
			if len(params) > 0 {
				reqURL += "?" + params.Encode()
			}

			resp, err := http.Get(reqURL)
			if err != nil {
				logger.Error("Failed to list sessions", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logger.Error("List sessions failed", "status", resp.Status)
				return
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Error("Failed to decode response", "error", err)
				return
			}

			sessions, ok := result["sessions"].([]interface{})
			if !ok || len(sessions) == 0 {
				logger.Info("No sessions found")
				return
			}

			logger.Info("Available Sessions:")
			logger.Info("=" + strings.Repeat("=", 79))
			for _, s := range sessions {
				session := s.(map[string]interface{})
				logger.Info("Session",
					"id", session["id"],
					"name", session["name"],
					"status", session["status"],
					"coordinator", session["coordinator_id"],
					"messages", session["message_count"],
					"agents", session["agent_count"],
					"tasks", session["task_count"],
				)
			}
		},
	}
}

// sessionAttachCmd reattaches to an existing session
func sessionAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name|id>",
		Short: "Attach to an existing session",
		Long: `Attach to a session that was previously created or detached.

This command:
- Finds the session by name or ID
- Loads all session state (messages, agents, tasks)
- Restores the session context
- Sets it as the current active session

Usage:
  flip2 session attach my-session-name
  flip2 session attach 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nameOrID := args[0]

			// Build request
			reqBody := map[string]string{
				"name_or_id": nameOrID,
			}

			jsonData, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				apiURL+"/api/sessions/attach",
				"application/json",
				strings.NewReader(string(jsonData)),
			)
			if err != nil {
				logger.Error("Failed to attach session", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logger.Error("Attach failed", "status", resp.Status)
				return
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Error("Failed to decode response", "error", err)
				return
			}

			session := result["session"].(map[string]interface{})
			logger.Info("Session attached successfully",
				"name", session["name"],
				"id", session["id"],
				"status", session["status"],
				"messages", session["message_count"],
				"agents", session["agent_count"],
				"tasks", session["task_count"],
			)

			// Print session details
			logger.Info("Session Details:")
			logger.Info("  Name:        %v", "value", session["name"])
			logger.Info("  ID:          %v", "value", session["id"])
			logger.Info("  Status:      %v", "value", session["status"])
			logger.Info("  Created:     %v", "value", session["created_at"])
			logger.Info("  Started:     %v", "value", session["started_at"])
			logger.Info("  Coordinator: %v", "value", session["coordinator_id"])
			logger.Info("  Messages:    %v", "value", session["message_count"])
			logger.Info("  Agents:      %v", "value", session["agent_count"])
			logger.Info("  Tasks:       %v", "value", session["task_count"])
		},
	}
}

// sessionStartCmd creates and starts a new session
func sessionStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a new session",
		Long: `Create and start a new FLIP2 session.

A session is a bounded execution context where agents collaborate.

Usage:
  flip2 session start my-analysis
  flip2 session start research-project --description "Market research"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			description, _ := cmd.Flags().GetString("description")
			coordinatorID, _ := cmd.Flags().GetString("coordinator")

			if coordinatorID == "" {
				coordinatorID = "cli-coordinator"
			}

			// Build request
			reqBody := map[string]string{
				"name":         name,
				"coordinator":  coordinatorID,
				"description":  description,
			}

			jsonData, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				apiURL+"/api/sessions/start",
				"application/json",
				strings.NewReader(string(jsonData)),
			)
			if err != nil {
				logger.Error("Failed to start session", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				logger.Error("Session creation failed", "status", resp.Status)
				return
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Error("Failed to decode response", "error", err)
				return
			}

			session := result["session"].(map[string]interface{})
			logger.Info("Session started successfully",
				"name", session["name"],
				"id", session["id"],
			)
		},
	}

	cmd.Flags().String("description", "", "Session description")
	cmd.Flags().String("coordinator", "", "Coordinator ID (default: cli-coordinator)")

	return cmd
}

// sessionStopCmd terminates a session
func sessionStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <session-id>",
		Short: "Stop a running session",
		Long: `Terminate a session and save its final state.

Usage:
  flip2 session stop my-session-id`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sessionID := args[0]
			finalStatus, _ := cmd.Flags().GetString("status")

			if finalStatus == "" {
				finalStatus = "completed"
			}

			// Build request
			reqBody := map[string]string{
				"session_id":   sessionID,
				"final_status": finalStatus,
			}

			jsonData, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				apiURL+"/api/sessions/stop",
				"application/json",
				strings.NewReader(string(jsonData)),
			)
			if err != nil {
				logger.Error("Failed to stop session", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logger.Error("Stop session failed", "status", resp.Status)
				return
			}

			logger.Info("Session stopped successfully", "session_id", sessionID)
		},
	}

	cmd.Flags().String("status", "completed", "Final status (completed, failed, cancelled)")

	return cmd
}
