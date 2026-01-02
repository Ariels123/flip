// Package config provides FLIP2.md parsing and configuration management.
package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// FLIP2MDParser handles parsing of FLIP2.md configuration files.
// It extracts YAML-like metadata, markdown tables, and structured sections
// and populates a ProjectConfig struct according to CFG-001 schema.
type FLIP2MDParser struct {
	filePath string
	content  string
	config   *ProjectConfig
}

// NewFLIP2MDParser creates a new parser instance for a FLIP2.md file.
func NewFLIP2MDParser(filePath string) *FLIP2MDParser {
	return &FLIP2MDParser{
		filePath: filePath,
		config:   &ProjectConfig{},
	}
}

// Parse loads and parses the FLIP2.md file, returning a ProjectConfig.
// It performs:
// 1. File reading with error handling
// 2. Metadata extraction from header
// 3. Section-by-section parsing
// 4. Schema validation
func (p *FLIP2MDParser) Parse() (*ProjectConfig, error) {
	// Initialize empty slices
	p.config.Agents = []AgentRole{}
	p.config.Commands = []Command{}
	p.config.Routes = []Route{}
	p.config.Context = ContextConfig{AutoLoadFiles: []ContextFile{}}

	// Read file
	data, err := os.ReadFile(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read FLIP2.md file at %q: %w", p.filePath, err)
	}

	p.content = string(data)

	// Parse header metadata
	if err := p.parseMetadata(); err != nil {
		return nil, fmt.Errorf("metadata extraction failed: %w", err)
	}

	// Parse all sections
	if err := p.parseSections(); err != nil {
		return nil, fmt.Errorf("section parsing failed: %w", err)
	}

	// Validate schema
	if result := ValidateProjectConfig(p.config); !result.Valid {
		return nil, fmt.Errorf("schema validation failed: %s", result.String())
	}

	return p.config, nil
}

// parseMetadata extracts YAML-like header metadata from the FLIP2.md file.
// Expects format:
// **Project:** ProjectName
// **Version:** X.Y.Z
// **Coordinator:** agent-id
// **Last Updated:** timestamp
func (p *FLIP2MDParser) parseMetadata() error {
	lines := strings.Split(p.content, "\n")

	projectRe := regexp.MustCompile(`^\*\*Project:\*\*\s+(.+?)(?:\s|$)`)
	versionRe := regexp.MustCompile(`^\*\*Version:\*\*\s+(.+?)(?:\s|$)`)
	coordRe := regexp.MustCompile(`^\*\*Coordinator:\*\*\s+(.+?)(?:\s|$)`)
	updatedRe := regexp.MustCompile(`^\*\*Last Updated:\*\*\s+(.+?)(?:\s|$)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if match := projectRe.FindStringSubmatch(line); match != nil {
			p.config.Project = strings.TrimSpace(match[1])
		} else if match := versionRe.FindStringSubmatch(line); match != nil {
			p.config.Version = strings.TrimSpace(match[1])
		} else if match := coordRe.FindStringSubmatch(line); match != nil {
			p.config.Coordinator = strings.TrimSpace(match[1])
		} else if match := updatedRe.FindStringSubmatch(line); match != nil {
			p.config.LastUpdated = strings.TrimSpace(match[1])
		}
	}

	// Project name is required
	if p.config.Project == "" {
		return fmt.Errorf("required metadata field 'Project' not found")
	}

	return nil
}

// parseSections identifies and parses major sections (## SectionName).
// Supported sections: Agents, Commands, Routing, Context
func (p *FLIP2MDParser) parseSections() error {
	sectionRe := regexp.MustCompile(`(?m)^##\s+(.+?)$`)
	matches := sectionRe.FindAllStringSubmatchIndex(p.content, -1)

	if len(matches) == 0 {
		return fmt.Errorf("no configuration sections found in FLIP2.md")
	}

	for i, match := range matches {
		sectionName := strings.TrimSpace(p.content[match[2]:match[3]])
		sectionStart := match[1]

		var sectionEnd int
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		} else {
			sectionEnd = len(p.content)
		}

		sectionContent := p.content[sectionStart:sectionEnd]

		switch strings.ToLower(sectionName) {
		case "agents":
			if err := p.parseAgentsSection(sectionContent); err != nil {
				return fmt.Errorf("agents section: %w", err)
			}
		case "commands":
			if err := p.parseCommandsSection(sectionContent); err != nil {
				return fmt.Errorf("commands section: %w", err)
			}
		case "routing":
			if err := p.parseRoutingSection(sectionContent); err != nil {
				return fmt.Errorf("routing section: %w", err)
			}
		case "context":
			if err := p.parseContextSection(sectionContent); err != nil {
				return fmt.Errorf("context section: %w", err)
			}
		}
	}

	return nil
}

// parseAgentsSection extracts agent role definitions.
// Expects format:
// ### Agent Role: RoleName
// - **ID Pattern:** `pattern`
// - **Model:** model-name
// - **Capabilities:** `cap1, cap2`
// etc.
func (p *FLIP2MDParser) parseAgentsSection(section string) error {
	agentRe := regexp.MustCompile(`(?m)^###\s+Agent Role:\s+(.+?)$`)
	matches := agentRe.FindAllStringSubmatchIndex(section, -1)

	if len(matches) == 0 {
		// No agents is valid
		return nil
	}

	for i, match := range matches {
		agentName := strings.TrimSpace(section[match[2]:match[3]])

		var agentEnd int
		if i+1 < len(matches) {
			agentEnd = matches[i+1][0]
		} else {
			agentEnd = len(section)
		}

		agentContent := section[match[0]:agentEnd]
		agent, err := p.parseAgentRole(agentName, agentContent)
		if err != nil {
			return fmt.Errorf("agent %q: %w", agentName, err)
		}

		p.config.Agents = append(p.config.Agents, agent)
	}

	return nil
}

// parseAgentRole extracts all fields from a single agent definition.
func (p *FLIP2MDParser) parseAgentRole(name string, content string) (AgentRole, error) {
	agent := AgentRole{Name: name}

	patterns := map[string]*regexp.Regexp{
		"id_pattern":          regexp.MustCompile(`^\s*-\s*\*\*ID Pattern:\*\*\s*` + "`" + `(.+?)` + "`"),
		"model":               regexp.MustCompile(`^\s*-\s*\*\*Model:\*\*\s+(.+?)(?:\s|$)`),
		"capabilities":        regexp.MustCompile(`^\s*-\s*\*\*Capabilities:\*\*\s*` + "`" + `(.+?)` + "`"),
		"permissions":         regexp.MustCompile(`^\s*-\s*\*\*Permissions:\*\*\s*` + "`" + `(.+?)` + "`"),
		"max_concurrent":      regexp.MustCompile(`^\s*-\s*\*\*Max Concurrent Tasks:\*\*\s*(\d+)`),
		"escalation_required": regexp.MustCompile(`^\s*-\s*\*\*Escalation Required For:\*\*\s*` + "`" + `(.+?)` + "`"),
		"cost_budget":         regexp.MustCompile(`^\s*-\s*\*\*Cost Budget \(USD/hour\):\*\*\s*([\d.]+)`),
		"description":         regexp.MustCompile(`^\s*-\s*\*\*Description:\*\*\s+(.+?)$`),
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
			if val, err := strconv.Atoi(strings.TrimSpace(match[1])); err == nil {
				agent.MaxConcurrentTasks = val
			}
		} else if match := patterns["escalation_required"].FindStringSubmatch(line); match != nil {
			agent.EscalationRequired = parseCommaSeparatedList(match[1])
		} else if match := patterns["cost_budget"].FindStringSubmatch(line); match != nil {
			if val, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64); err == nil {
				agent.CostBudgetPerHour = val
			}
		} else if match := patterns["description"].FindStringSubmatch(line); match != nil {
			agent.Description = strings.TrimSpace(match[1])
		}
	}

	return agent, nil
}

// parseCommandsSection extracts custom slash command definitions.
// Expects format:
// ### Command: /command-name
// - **Aliases:** `alias1, alias2`
// - **Handler:** `handler-id`
// - **Args:** `<arg1> [--flag]`
// etc.
func (p *FLIP2MDParser) parseCommandsSection(section string) error {
	cmdRe := regexp.MustCompile(`(?m)^###\s+Command:\s+(/\S+)`)
	matches := cmdRe.FindAllStringSubmatchIndex(section, -1)

	if len(matches) == 0 {
		// No commands is valid
		return nil
	}

	for i, match := range matches {
		cmdName := strings.TrimSpace(section[match[2]:match[3]])

		var cmdEnd int
		if i+1 < len(matches) {
			cmdEnd = matches[i+1][0]
		} else {
			cmdEnd = len(section)
		}

		cmdContent := section[match[0]:cmdEnd]
		cmd, err := p.parseCommand(cmdName, cmdContent)
		if err != nil {
			return fmt.Errorf("command %q: %w", cmdName, err)
		}

		p.config.Commands = append(p.config.Commands, cmd)
	}

	return nil
}

// parseCommand extracts all fields from a single command definition.
func (p *FLIP2MDParser) parseCommand(name string, content string) (Command, error) {
	cmd := Command{Name: name}

	patterns := map[string]*regexp.Regexp{
		"aliases":           regexp.MustCompile(`^\s*-\s*\*\*Aliases:\*\*\s*` + "`" + `(.+?)` + "`"),
		"handler":           regexp.MustCompile(`^\s*-\s*\*\*Handler:\*\*\s*` + "`" + `(.+?)` + "`"),
		"args":              regexp.MustCompile(`^\s*-\s*\*\*Args:\*\*\s*` + "`" + `(.+?)` + "`"),
		"description":       regexp.MustCompile(`^\s*-\s*\*\*Description:\*\*\s+(.+?)$`),
		"requires_approval": regexp.MustCompile(`^\s*-\s*\*\*Requires Approval:\*\*\s+(.+?)$`),
		"allowed_roles":     regexp.MustCompile(`^\s*-\s*\*\*Allowed Roles:\*\*\s*` + "`" + `(.+?)` + "`"),
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

// parseRoutingSection extracts task routing rule definitions.
// Expects format:
// ### Route: RouteName
// - **When:** `condition`
// - **Route To:** `destination`
// - **Reason:** explanation
// - **Cost Impact:** `impact-value`
func (p *FLIP2MDParser) parseRoutingSection(section string) error {
	routeRe := regexp.MustCompile(`(?m)^###\s+Route:\s+(.+?)$`)
	matches := routeRe.FindAllStringSubmatchIndex(section, -1)

	if len(matches) == 0 {
		// No routes is valid
		return nil
	}

	for i, match := range matches {
		routeName := strings.TrimSpace(section[match[2]:match[3]])

		var routeEnd int
		if i+1 < len(matches) {
			routeEnd = matches[i+1][0]
		} else {
			routeEnd = len(section)
		}

		routeContent := section[match[0]:routeEnd]
		route, err := p.parseRoute(routeName, routeContent)
		if err != nil {
			return fmt.Errorf("route %q: %w", routeName, err)
		}

		p.config.Routes = append(p.config.Routes, route)
	}

	return nil
}

// parseRoute extracts all fields from a single routing definition.
func (p *FLIP2MDParser) parseRoute(name string, content string) (Route, error) {
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
			// Handle +/- prefixes
			costStr = strings.TrimPrefix(costStr, "+")
			if val, err := strconv.ParseFloat(costStr, 64); err == nil {
				route.CostImpact = val
			}
		}
	}

	return route, nil
}

// parseContextSection extracts context file configuration.
// Expects format:
// ### Auto-Load Files
// - `./path/to/file` - Description (weight: high|medium|low)
func (p *FLIP2MDParser) parseContextSection(section string) error {
	// Find Auto-Load Files section
	filesRe := regexp.MustCompile(`(?m)^###\s+Auto-Load Files$`)
	if !filesRe.MatchString(section) {
		// Context section is optional
		return nil
	}

	// Parse file entries
	fileRe := regexp.MustCompile(`(?m)^-\s+` + "`" + `(.+?)` + "`" + `\s*-\s+(.+?)(?:\s*\(weight:\s+(.+?)\))?$`)
	matches := fileRe.FindAllStringSubmatch(section, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		contextFile := ContextFile{
			Path:        strings.TrimSpace(match[1]),
			Description: strings.TrimSpace(match[2]),
			Weight:      "medium", // default weight
		}

		// Extract weight if present
		if len(match) > 3 && match[3] != "" {
			contextFile.Weight = strings.TrimSpace(match[3])
		}

		p.config.Context.AutoLoadFiles = append(p.config.Context.AutoLoadFiles, contextFile)
	}

	return nil
}
