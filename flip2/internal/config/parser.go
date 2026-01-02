// Package config provides FLIP2.md parsing and configuration management.
package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

)

// ProjectConfig represents the complete FLIP2.md configuration for a project.
type ProjectConfig struct {
	Project     string
	Version     string
	Coordinator string
	LastUpdated string

	Agents  []AgentRole
	Commands []Command
	Routes  []Route
	Context ContextConfig
}

// AgentRole defines a custom agent role with permissions and capabilities.
type AgentRole struct {
	Name              string
	IDPattern         string
	Model             string
	Capabilities      []string
	Permissions       []string
	MaxConcurrentTasks int
	EscalationRequired []string
	CostBudgetPerHour  float64
	Description       string
}

// Command represents a registered slash command.
type Command struct {
	Name            string
	Aliases         []string
	Handler         string
	Args            string
	Description     string
	RequiresApproval bool
	AllowedRoles    []string
}

// Route defines a task routing rule.
type Route struct {
	Name       string
	Condition  string
	RouteTo    string
	Reason     string
	CostImpact float64
}

// ContextConfig specifies auto-loaded files.
type ContextConfig struct {
	AutoLoadFiles []ContextFile
}

// ContextFile represents a file to be auto-loaded.
type ContextFile struct {
	Path        string
	Description string
	Weight      string // "low", "medium", "high"
}

// ParseFLIP2MD parses a FLIP2.md file and returns a ProjectConfig.
func ParseFLIP2MD(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read FLIP2.md file: %w", err)
	}

	content := string(data)

	config := &ProjectConfig{
		Agents:   []AgentRole{},
		Commands: []Command{},
		Routes:   []Route{},
		Context:  ContextConfig{AutoLoadFiles: []ContextFile{}},
	}

	// Parse header metadata
	if err := parseHeader(content, config); err != nil {
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Parse sections
	if err := parseSections(content, config); err != nil {
		return nil, fmt.Errorf("failed to parse sections: %w", err)
	}

	// Validate schema
	if err := validateSchema(config); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	return config, nil
}

// parseHeader extracts metadata from the FLIP2.md header
func parseHeader(content string, config *ProjectConfig) error {
	lines := strings.Split(content, "\n")

	projectRe := regexp.MustCompile(`^\*\*Project:\*\*\s+(.+)$`)
	versionRe := regexp.MustCompile(`^\*\*Version:\*\*\s+(.+)$`)
	coordRe := regexp.MustCompile(`^\*\*Coordinator:\*\*\s+(.+)$`)
	updatedRe := regexp.MustCompile(`^\*\*Last Updated:\*\*\s+(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if match := projectRe.FindStringSubmatch(line); match != nil {
			config.Project = strings.TrimSpace(match[1])
		}
		if match := versionRe.FindStringSubmatch(line); match != nil {
			config.Version = strings.TrimSpace(match[1])
		}
		if match := coordRe.FindStringSubmatch(line); match != nil {
			config.Coordinator = strings.TrimSpace(match[1])
		}
		if match := updatedRe.FindStringSubmatch(line); match != nil {
			config.LastUpdated = strings.TrimSpace(match[1])
		}
	}

	return nil
}

// parseSections extracts all major sections from FLIP2.md
func parseSections(content string, config *ProjectConfig) error {
	// Split into sections
	sectionRe := regexp.MustCompile(`(?m)^## (.+)$`)
	matches := sectionRe.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return fmt.Errorf("no sections found in FLIP2.md")
	}

	for i, match := range matches {
		sectionName := strings.TrimSpace(content[match[2]:match[3]])
		sectionStart := match[1]

		var sectionEnd int
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		} else {
			sectionEnd = len(content)
		}

		sectionContent := content[sectionStart:sectionEnd]

		switch strings.ToLower(sectionName) {
		case "agents":
			if err := parseAgents(sectionContent, config); err != nil {
				return err
			}
		case "commands":
			if err := parseCommands(sectionContent, config); err != nil {
				return err
			}
		case "routing":
			if err := parseRouting(sectionContent, config); err != nil {
				return err
			}
		case "context":
			if err := parseContext(sectionContent, config); err != nil {
				return err
			}
		}
	}

	return nil
}

// parseAgents extracts agent role definitions
func parseAgents(section string, config *ProjectConfig) error {
	// Find all ### Agent Role: entries
	agentRe := regexp.MustCompile(`(?m)^### Agent Role:\s+(.+)$`)
	matches := agentRe.FindAllStringSubmatchIndex(section, -1)

	for i, match := range matches {
		agentName := strings.TrimSpace(section[match[2]:match[3]])

		var agentEnd int
		if i+1 < len(matches) {
			agentEnd = matches[i+1][0]
		} else {
			agentEnd = len(section)
		}

		agentContent := section[match[0]:agentEnd]
		agent, err := parseAgentRole(agentName, agentContent)
		if err != nil {
			return fmt.Errorf("failed to parse agent %q: %w", agentName, err)
		}

		config.Agents = append(config.Agents, agent)
	}

	return nil
}

// parseAgentRole extracts fields from a single agent definition
func parseAgentRole(name string, content string) (AgentRole, error) {
	agent := AgentRole{Name: name}

	// Define regex patterns for each field
	patterns := map[string]*regexp.Regexp{
		"id_pattern":           regexp.MustCompile(`^\s*-\s*\*\*ID Pattern:\*\*\s*` + "`" + `(.+?)` + "`"),
		"model":                regexp.MustCompile(`^\s*-\s*\*\*Model:\*\*\s+(.+?)(?:\s*\||\s*$)`),
		"capabilities":         regexp.MustCompile(`^\s*-\s*\*\*Capabilities:\*\*\s*` + "`" + `(.+?)` + "`"),
		"permissions":          regexp.MustCompile(`^\s*-\s*\*\*Permissions:\*\*\s*` + "`" + `(.+?)` + "`"),
		"max_concurrent":       regexp.MustCompile(`^\s*-\s*\*\*Max Concurrent Tasks:\*\*\s*(.+?)$`),
		"escalation_required":  regexp.MustCompile(`^\s*-\s*\*\*Escalation Required For:\*\*\s*` + "`" + `(.+?)` + "`"),
		"cost_budget":          regexp.MustCompile(`^\s*-\s*\*\*Cost Budget \(USD/hour\):\*\*\s*(.+?)$`),
		"description":          regexp.MustCompile(`^\s*-\s*\*\*Description:\*\*\s+(.+?)$`),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		if match := patterns["id_pattern"].FindStringSubmatch(line); match != nil {
			agent.IDPattern = strings.TrimSpace(match[1])
		} else if match := patterns["model"].FindStringSubmatch(line); match != nil {
			agent.Model = strings.TrimSpace(match[1])
		} else if match := patterns["capabilities"].FindStringSubmatch(line); match != nil {
			agent.Capabilities = parseCommaSeparatedList(match[1])
		} else if match := patterns["permissions"].FindStringSubmatch(line); match != nil {
			agent.Permissions = parseCommaSeparatedList(match[1])
		} else if match := patterns["max_concurrent"].FindStringSubmatch(line); match != nil {
			fmt.Sscanf(strings.TrimSpace(match[1]), "%d", &agent.MaxConcurrentTasks)
		} else if match := patterns["escalation_required"].FindStringSubmatch(line); match != nil {
			agent.EscalationRequired = parseCommaSeparatedList(match[1])
		} else if match := patterns["cost_budget"].FindStringSubmatch(line); match != nil {
			fmt.Sscanf(strings.TrimSpace(match[1]), "%f", &agent.CostBudgetPerHour)
		} else if match := patterns["description"].FindStringSubmatch(line); match != nil {
			agent.Description = strings.TrimSpace(match[1])
		}
	}

	return agent, nil
}

// parseCommands extracts slash command definitions
func parseCommands(section string, config *ProjectConfig) error {
	// Find all ### Command: entries
	cmdRe := regexp.MustCompile(`(?m)^### Command:\s+(/\S+)`)
	matches := cmdRe.FindAllStringSubmatchIndex(section, -1)

	for i, match := range matches {
		cmdName := strings.TrimSpace(section[match[2]:match[3]])

		var cmdEnd int
		if i+1 < len(matches) {
			cmdEnd = matches[i+1][0]
		} else {
			cmdEnd = len(section)
		}

		cmdContent := section[match[0]:cmdEnd]
		cmd, err := parseCommand(cmdName, cmdContent)
		if err != nil {
			return fmt.Errorf("failed to parse command %q: %w", cmdName, err)
		}

		config.Commands = append(config.Commands, cmd)
	}

	return nil
}

// parseCommand extracts fields from a single command definition
func parseCommand(name string, content string) (Command, error) {
	cmd := Command{Name: name}

	patterns := map[string]*regexp.Regexp{
		"aliases":             regexp.MustCompile(`^\s*-\s*\*\*Aliases:\*\*\s*` + "`" + `(.+?)` + "`"),
		"handler":             regexp.MustCompile(`^\s*-\s*\*\*Handler:\*\*\s*` + "`" + `(.+?)` + "`"),
		"args":                regexp.MustCompile(`^\s*-\s*\*\*Args:\*\*\s*` + "`" + `(.+?)` + "`"),
		"description":         regexp.MustCompile(`^\s*-\s*\*\*Description:\*\*\s+(.+?)$`),
		"requires_approval":   regexp.MustCompile(`^\s*-\s*\*\*Requires Approval:\*\*\s+(.+?)$`),
		"allowed_roles":       regexp.MustCompile(`^\s*-\s*\*\*Allowed Roles:\*\*\s*` + "`" + `(.+?)` + "`"),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		if match := patterns["aliases"].FindStringSubmatch(line); match != nil {
			cmd.Aliases = parseCommaSeparatedList(match[1])
		} else if match := patterns["handler"].FindStringSubmatch(line); match != nil {
			cmd.Handler = strings.TrimSpace(match[1])
		} else if match := patterns["args"].FindStringSubmatch(line); match != nil {
			cmd.Args = strings.TrimSpace(match[1])
		} else if match := patterns["description"].FindStringSubmatch(line); match != nil {
			cmd.Description = strings.TrimSpace(match[1])
		} else if match := patterns["requires_approval"].FindStringSubmatch(line); match != nil {
			approval := strings.ToLower(strings.TrimSpace(match[1]))
			cmd.RequiresApproval = approval == "yes" || approval == "true"
		} else if match := patterns["allowed_roles"].FindStringSubmatch(line); match != nil {
			cmd.AllowedRoles = parseCommaSeparatedList(match[1])
		}
	}

	return cmd, nil
}

// parseRouting extracts routing rule definitions
func parseRouting(section string, config *ProjectConfig) error {
	// Find all ### Route: entries
	routeRe := regexp.MustCompile(`(?m)^### Route:\s+(.+)$`)
	matches := routeRe.FindAllStringSubmatchIndex(section, -1)

	for i, match := range matches {
		routeName := strings.TrimSpace(section[match[2]:match[3]])

		var routeEnd int
		if i+1 < len(matches) {
			routeEnd = matches[i+1][0]
		} else {
			routeEnd = len(section)
		}

		routeContent := section[match[0]:routeEnd]
		route, err := parseRoute(routeName, routeContent)
		if err != nil {
			return fmt.Errorf("failed to parse route %q: %w", routeName, err)
		}

		config.Routes = append(config.Routes, route)
	}

	return nil
}

// parseRoute extracts fields from a single routing definition
func parseRoute(name string, content string) (Route, error) {
	route := Route{Name: name}

	patterns := map[string]*regexp.Regexp{
		"when":        regexp.MustCompile(`^\s*-\s*\*\*When:\*\*\s*` + "`" + `(.+?)` + "`"),
		"route_to":    regexp.MustCompile(`^\s*-\s*\*\*Route To:\*\*\s*` + "`" + `(.+?)` + "`"),
		"reason":      regexp.MustCompile(`^\s*-\s*\*\*Reason:\*\*\s+(.+?)$`),
		"cost_impact": regexp.MustCompile(`^\s*-\s*\*\*Cost Impact:\*\*\s*` + "`" + `(.+?)` + "`"),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		if match := patterns["when"].FindStringSubmatch(line); match != nil {
			route.Condition = strings.TrimSpace(match[1])
		} else if match := patterns["route_to"].FindStringSubmatch(line); match != nil {
			route.RouteTo = strings.TrimSpace(match[1])
		} else if match := patterns["reason"].FindStringSubmatch(line); match != nil {
			route.Reason = strings.TrimSpace(match[1])
		} else if match := patterns["cost_impact"].FindStringSubmatch(line); match != nil {
			costStr := strings.TrimSpace(match[1])
			fmt.Sscanf(costStr, "%f", &route.CostImpact)
		}
	}

	return route, nil
}

// parseContext extracts context file configuration
func parseContext(section string, config *ProjectConfig) error {
	// Find Auto-Load Files section
	filesRe := regexp.MustCompile(`(?m)^### Auto-Load Files$`)
	if !filesRe.MatchString(section) {
		return nil // Context section is optional
	}

	// Find all file entries
	fileRe := regexp.MustCompile(`(?m)^-\s+\` + "`" + `(.+?)\` + "`" + `\s*-\s+(.+?)(?:\s*\(weight:\s+(.+?)\))?$`)
	matches := fileRe.FindAllStringSubmatch(section, -1)

	for _, match := range matches {
		contextFile := ContextFile{
			Path:        strings.TrimSpace(match[1]),
			Description: strings.TrimSpace(match[2]),
			Weight:      "medium", // default weight
		}

		if len(match) > 3 && match[3] != "" {
			contextFile.Weight = strings.TrimSpace(match[3])
		}

		config.Context.AutoLoadFiles = append(config.Context.AutoLoadFiles, contextFile)
	}

	return nil
}

// parseCommaSeparatedList splits a comma-separated string and trims whitespace
func parseCommaSeparatedList(input string) []string {
	items := strings.Split(input, ",")
	var result []string
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// validateSchema checks that the configuration meets schema requirements
func validateSchema(config *ProjectConfig) error {
	// Validate agents
	seenAgentIDs := make(map[string]bool)
	for _, agent := range config.Agents {
		if agent.IDPattern == "" {
			return fmt.Errorf("agent %q is missing ID Pattern", agent.Name)
		}
		if seenAgentIDs[agent.IDPattern] {
			return fmt.Errorf("duplicate agent ID pattern: %q", agent.IDPattern)
		}
		seenAgentIDs[agent.IDPattern] = true

		if agent.Model == "" {
			return fmt.Errorf("agent %q is missing Model", agent.Name)
		}
	}

	// Validate commands
	seenCmds := make(map[string]bool)
	for _, cmd := range config.Commands {
		if !strings.HasPrefix(cmd.Name, "/") {
			return fmt.Errorf("command %q must start with /", cmd.Name)
		}
		if seenCmds[cmd.Name] {
			return fmt.Errorf("duplicate command: %q", cmd.Name)
		}
		seenCmds[cmd.Name] = true

		if cmd.Handler == "" {
			return fmt.Errorf("command %q is missing Handler", cmd.Name)
		}
	}

	// Validate routes
	for _, route := range config.Routes {
		if route.Condition == "" {
			return fmt.Errorf("route %q is missing When condition", route.Name)
		}
		if route.RouteTo == "" {
			return fmt.Errorf("route %q is missing Route To", route.Name)
		}
	}

	// Validate context file paths
	for _, file := range config.Context.AutoLoadFiles {
		if file.Path == "" {
			return fmt.Errorf("context file is missing path")
		}
		if file.Weight != "" && file.Weight != "low" && file.Weight != "medium" && file.Weight != "high" {
			return fmt.Errorf("invalid weight %q for context file %q, must be low/medium/high", file.Weight, file.Path)
		}
	}

	return nil
}
