package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MatchThreshold is the minimum confidence score required for a tool match.
// Matches below this threshold are filtered out.
const MatchThreshold = 0.7

// TaskRequirement describes a task that needs to be fulfilled by a tool.
//
// A TaskRequirement contains criteria for matching against available tools,
// including natural language descriptions, required capabilities, and
// input parameter specifications.
type TaskRequirement struct {
	// Name is a human-readable name for the task.
	// Example: "read configuration file"
	Name string

	// Description is a detailed description of what needs to be done.
	// This is used for semantic matching against tool descriptions.
	// Example: "I need to read the contents of a configuration file"
	Description string

	// RequiredCapabilities is a list of capability keywords the tool must have.
	// All specified capabilities must be present in the matched tool.
	// Example: ["filesystem", "read"]
	RequiredCapabilities []string

	// InputParameters specifies the required input parameters the tool must accept.
	// Maps parameter name to its JSON Schema type.
	// Example: map[string]string{"path": "string", "encoding": "string"}
	InputParameters map[string]string

	// Metadata contains additional context about the task.
	Metadata map[string]any
}

// MatchableTool represents a tool from an MCP server with extended matching information.
// This is distinct from MCPTool in discovery.go which is used for tool discovery.
type MatchableTool struct {
	// Tool is the underlying MCP tool definition.
	Tool Tool

	// ServerName is the name of the MCP server providing this tool.
	ServerName string

	// Capabilities are inferred capability tags for this tool.
	// Derived from tool name, description, and annotations.
	Capabilities []string

	// ParsedSchema is the parsed JSON Schema for the tool's input.
	ParsedSchema *SchemaInfo
}

// SchemaInfo represents parsed information from a JSON Schema.
type SchemaInfo struct {
	// Type is the schema type ("object", "string", "number", etc.).
	Type string

	// Properties maps property names to their types (for object schemas).
	Properties map[string]string

	// Required is a list of required property names.
	Required []string
}

// MatchResult represents the result of matching a requirement to a tool.
type MatchResult struct {
	// Tool is the matched tool.
	Tool *MatchableTool

	// Score is the overall match score (0-1).
	// 1.0 = perfect match, 0.0 = no match.
	Score float64

	// ComponentScores breaks down the score by component.
	ComponentScores *MatchScoreBreakdown

	// Reasoning explains why this score was assigned.
	Reasoning string
}

// MatchScoreBreakdown provides detailed scoring information for tool matching.
// This is distinct from ScoreBreakdown in router.go which is used for routing decisions.
type MatchScoreBreakdown struct {
	// NameSimilarity is the fuzzy match score for tool name vs description (0-1).
	NameSimilarity float64

	// DescriptionMatch is the keyword match score in tool description (0-1).
	DescriptionMatch float64

	// CapabilityMatch is the proportion of required capabilities present (0-1).
	CapabilityMatch float64

	// InputSchemaMatch is the proportion of required inputs supported (0-1).
	InputSchemaMatch float64
}

// MatchTool attempts to match a task requirement to a tool.
//
// The matching algorithm considers multiple factors:
// 1. Name similarity using fuzzy matching
// 2. Description keyword matching
// 3. Required capability presence
// 4. Input schema compatibility
//
// Returns the match result with a confidence score. If the score is below
// MatchThreshold (0.7), the match should be treated as insufficient.
//
// Returns an error only for invalid input (nil pointers, unparseable schemas).
func MatchTool(requirement *TaskRequirement, tool *MatchableTool) (*MatchResult, error) {
	if requirement == nil || tool == nil {
		return nil, fmt.Errorf("requirement and tool must not be nil")
	}

	// Parse the tool's input schema if not already parsed
	if tool.ParsedSchema == nil {
		schema, err := parseSchema(tool.Tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tool schema: %w", err)
		}
		tool.ParsedSchema = schema
	}

	// Calculate component scores
	nameSimilarity := calculateNameSimilarity(
		requirement.Description,
		tool.Tool.Name,
		tool.Tool.Description,
	)

	descriptionMatch := calculateDescriptionMatch(
		requirement.Description,
		tool.Tool.Description,
	)

	capabilityMatch := calculateCapabilityMatch(
		requirement.RequiredCapabilities,
		tool.Capabilities,
	)

	inputSchemaMatch := calculateInputSchemaMatch(
		requirement.InputParameters,
		tool.ParsedSchema,
	)

	// Weight the components and calculate overall score
	// Weights are chosen to balance different matching criteria
	overallScore := (nameSimilarity * 0.25) +
		(descriptionMatch * 0.25) +
		(capabilityMatch * 0.25) +
		(inputSchemaMatch * 0.25)

	breakdown := &MatchScoreBreakdown{
		NameSimilarity:   nameSimilarity,
		DescriptionMatch: descriptionMatch,
		CapabilityMatch:  capabilityMatch,
		InputSchemaMatch: inputSchemaMatch,
	}

	reasoning := buildReasoningString(requirement, tool, breakdown)

	return &MatchResult{
		Tool:               tool,
		Score:              overallScore,
		ComponentScores:    breakdown,
		Reasoning:          reasoning,
	}, nil
}

// MatchTools attempts to match a requirement against multiple tools.
//
// Returns all matches with scores >= MatchThreshold, sorted by score descending.
// If no matches meet the threshold, returns an empty slice.
func MatchTools(requirement *TaskRequirement, tools []*MatchableTool) ([]*MatchResult, error) {
	if requirement == nil {
		return nil, fmt.Errorf("requirement must not be nil")
	}

	var matches []*MatchResult

	for _, tool := range tools {
		if tool == nil {
			continue
		}

		result, err := MatchTool(requirement, tool)
		if err != nil {
			// Skip tools with unparseable schemas
			continue
		}

		// Only include matches above threshold
		if result.Score >= MatchThreshold {
			matches = append(matches, result)
		}
	}

	// Sort by score descending (highest confidence first)
	// Using bubble sort for simplicity (small lists typical)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches, nil
}

// calculateNameSimilarity uses fuzzy string matching to compare the requirement
// description against the tool name and description.
func calculateNameSimilarity(requirement, toolName, toolDescription string) float64 {
	if requirement == "" || (toolName == "" && toolDescription == "") {
		return 0.0
	}

	// Normalize strings for comparison
	reqNorm := normalizeString(requirement)
	nameNorm := normalizeString(toolName)
	descNorm := normalizeString(toolDescription)

	// Extract keywords from requirement (prioritize action verbs)
	keywords := extractKeywords(reqNorm)

	// Score based on keyword matches
	if len(keywords) == 0 {
		return 0.0
	}

	matchCount := 0
	for _, keyword := range keywords {
		// Check for exact word match in tool name or description
		if containsWord(nameNorm, keyword) || containsWord(descNorm, keyword) {
			matchCount++
		}
	}

	// Return proportion of matched keywords
	return float64(matchCount) / float64(len(keywords))
}

// calculateDescriptionMatch scores how well the tool description matches
// the requirement description using keyword analysis.
func calculateDescriptionMatch(requirement, toolDescription string) float64 {
	if requirement == "" || toolDescription == "" {
		return 0.0
	}

	reqNorm := normalizeString(requirement)
	descNorm := normalizeString(toolDescription)

	// Extract keywords from requirement
	reqKeywords := extractKeywords(reqNorm)
	if len(reqKeywords) == 0 {
		return 0.0
	}

	// Count matches in tool description
	matchCount := 0
	for _, keyword := range reqKeywords {
		if containsWord(descNorm, keyword) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(reqKeywords))
}

// calculateCapabilityMatch scores based on required capabilities presence.
// Returns 1.0 if all required capabilities are present, otherwise returns
// the proportion of capabilities found.
func calculateCapabilityMatch(required, available []string) float64 {
	if len(required) == 0 {
		return 1.0 // No requirements = perfect match
	}

	if len(available) == 0 {
		return 0.0 // Required capabilities but none available
	}

	matchCount := 0
	for _, req := range required {
		if containsCapability(available, req) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(required))
}

// calculateInputSchemaMatch scores based on input schema compatibility.
// Returns 1.0 if all required parameters are present and properly typed,
// otherwise returns the proportion of parameters found and correctly typed.
func calculateInputSchemaMatch(required map[string]string, schema *SchemaInfo) float64 {
	if len(required) == 0 {
		return 1.0 // No requirements = perfect match
	}

	if schema == nil || schema.Properties == nil {
		return 0.0 // Schema not available
	}

	matchCount := 0
	for paramName, paramType := range required {
		if schemaType, exists := schema.Properties[paramName]; exists {
			// Type must match (exact match or compatible)
			if schemaType == paramType || isCompatibleType(paramType, schemaType) {
				matchCount++
			}
		}
	}

	return float64(matchCount) / float64(len(required))
}

// normalizeString converts a string to lowercase and removes extra whitespace.
func normalizeString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)
	// Remove extra whitespace
	return strings.TrimSpace(s)
}

// extractKeywords extracts significant words from a string.
// Filters out common stopwords.
func extractKeywords(s string) []string {
	// Common English stopwords to ignore
	stopwords := map[string]bool{
		"the":  true,
		"a":    true,
		"an":   true,
		"and":  true,
		"or":   true,
		"but":  true,
		"in":   true,
		"on":   true,
		"at":   true,
		"to":   true,
		"from": true,
		"is":   true,
		"are":  true,
		"be":   true,
		"been": true,
		"for":  true,
		"of":   true,
		"with": true,
		"that": true,
		"this": true,
		"i":    true,
		"it":   true,
		"by":   true,
		"as":   true,
		"if":   true,
		"need": true,
		"want": true,
		"must": true,
		"have": true,
		"has":  true,
	}

	words := strings.Fields(s)
	var keywords []string

	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 2 && !stopwords[word] { // Only significant words
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// containsWord checks if a normalized string contains a complete word.
// Uses fuzzy matching to handle singular/plural and related word forms.
func containsWord(text, word string) bool {
	words := strings.Fields(text)
	for _, w := range words {
		// Remove punctuation for comparison
		w = strings.Trim(w, ".,!?;:")

		// Exact match
		if w == word {
			return true
		}

		// Fuzzy match: substring matching or word stem matching
		// Check if word is contained in w or vice versa (for plurals/variants)
		if strings.Contains(w, word) || strings.Contains(word, w) {
			// But require at least 70% similarity to avoid false positives
			if len(word) > 0 && len(w) > 0 {
				// Simple heuristic: if one is a substring of the other and they're close in length
				shorter := len(word)
				longer := len(w)
				if shorter > longer {
					shorter, longer = longer, shorter
				}
				similarity := float64(shorter) / float64(longer)
				if similarity >= 0.7 {
					return true
				}
			}
		}
	}
	return false
}

// containsCapability checks if a capability is present in a list.
// Case-insensitive comparison.
func containsCapability(capabilities []string, required string) bool {
	reqNorm := strings.ToLower(required)
	for _, cap := range capabilities {
		if strings.ToLower(cap) == reqNorm {
			return true
		}
	}
	return false
}

// isCompatibleType checks if two JSON Schema types are compatible.
func isCompatibleType(required, available string) bool {
	req := strings.ToLower(required)
	avail := strings.ToLower(available)

	if req == avail {
		return true
	}

	// Define compatible type pairs
	compatible := map[string]map[string]bool{
		"string": {"string": true},
		"number": {"number": true, "integer": true},
		"integer": {"integer": true, "number": true},
		"boolean": {"boolean": true},
		"object": {"object": true},
		"array": {"array": true},
		"any": {"string": true, "number": true, "integer": true, "boolean": true, "object": true, "array": true},
	}

	if types, ok := compatible[req]; ok {
		return types[avail]
	}

	return false
}

// parseSchema extracts structured information from a JSON Schema.
// Returns nil if the schema is empty or invalid.
func parseSchema(rawSchema json.RawMessage) (*SchemaInfo, error) {
	if len(rawSchema) == 0 {
		return &SchemaInfo{}, nil
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(rawSchema, &schemaMap); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}

	info := &SchemaInfo{
		Properties: make(map[string]string),
		Required:   []string{},
	}

	// Extract type
	if t, ok := schemaMap["type"].(string); ok {
		info.Type = t
	}

	// Extract properties for object schemas
	if props, ok := schemaMap["properties"].(map[string]any); ok {
		for propName, propDef := range props {
			if propMap, ok := propDef.(map[string]any); ok {
				if propType, ok := propMap["type"].(string); ok {
					info.Properties[propName] = propType
				}
			}
		}
	}

	// Extract required fields
	if required, ok := schemaMap["required"].([]any); ok {
		for _, r := range required {
			if fieldName, ok := r.(string); ok {
				info.Required = append(info.Required, fieldName)
			}
		}
	}

	return info, nil
}

// buildReasoningString generates a human-readable explanation of the match.
func buildReasoningString(requirement *TaskRequirement, tool *MatchableTool, scores *MatchScoreBreakdown) string {
	var parts []string

	// Start with tool identification
	parts = append(parts, fmt.Sprintf(
		"Matched tool '%s' from server '%s'",
		tool.Tool.Name,
		tool.ServerName,
	))

	// Add score breakdowns
	parts = append(parts, fmt.Sprintf(
		"(name: %.0f%%, description: %.0f%%, capabilities: %.0f%%, schema: %.0f%%)",
		scores.NameSimilarity*100,
		scores.DescriptionMatch*100,
		scores.CapabilityMatch*100,
		scores.InputSchemaMatch*100,
	))

	// Add strengths
	var strengths []string
	if scores.NameSimilarity >= 0.8 {
		strengths = append(strengths, "strong name match")
	}
	if scores.DescriptionMatch >= 0.8 {
		strengths = append(strengths, "clear description match")
	}
	if scores.CapabilityMatch >= 0.9 {
		strengths = append(strengths, "all capabilities present")
	}
	if scores.InputSchemaMatch >= 0.9 {
		strengths = append(strengths, "input schema fully compatible")
	}

	if len(strengths) > 0 {
		parts = append(parts, "Strengths: "+strings.Join(strengths, ", "))
	}

	return strings.Join(parts, ". ")
}

// InferCapabilities analyzes a tool to infer its capability tags.
// This is used during tool registration to populate the Capabilities field.
func InferCapabilities(tool *Tool, toolName string) []string {
	var caps []string
	capMap := make(map[string]bool) // Use map to avoid duplicates

	// Infer from tool name
	for _, cap := range inferFromName(toolName) {
		capMap[cap] = true
	}

	// Infer from description
	for _, cap := range inferFromDescription(tool.Description) {
		capMap[cap] = true
	}

	// Infer from annotations
	if tool.Annotations != nil {
		if tool.Annotations.ReadOnlyHint {
			capMap["readonly"] = true
		}
		if tool.Annotations.DestructiveHint {
			capMap["destructive"] = true
		}
		if tool.Annotations.IdempotentHint {
			capMap["idempotent"] = true
		}
		if tool.Annotations.OpenWorldHint {
			capMap["openworld"] = true
		}
	}

	// Convert map to slice
	for cap := range capMap {
		caps = append(caps, cap)
	}

	return caps
}

// inferFromName extracts capability keywords from a tool name.
func inferFromName(name string) []string {
	caps := make([]string, 0)
	capSet := make(map[string]bool) // To avoid duplicates

	// Common tool name patterns
	patterns := map[string]string{
		"read":       "read",
		"write":      "write",
		"create":     "create",
		"delete":     "delete",
		"update":     "update",
		"list":       "list",
		"search":     "search",
		"execute":    "execute",
		"run":        "execute",
		"call":       "call",
		"get":        "read",
		"post":       "write",
		"put":        "write",
		"fetch":      "read",
		"query":      "query",
		"file":       "filesystem",
		"directory": "filesystem",
		"dir":        "filesystem",
		"web":        "web",
		"http":       "web",
		"api":        "api",
		"database":   "database",
		"db":         "database",
		"shell":      "execution",
		"command":    "execution",
		"code":       "execution",
		"git":        "vcs",
		"github":     "vcs",
	}

	nameLower := strings.ToLower(name)
	for pattern, capability := range patterns {
		if strings.Contains(nameLower, pattern) {
			// Add the mapped capability
			if !capSet[capability] {
				caps = append(caps, capability)
				capSet[capability] = true
			}

			// If pattern differs from capability, also add the original pattern
			if pattern != capability && !capSet[pattern] {
				caps = append(caps, pattern)
				capSet[pattern] = true
			}
		}
	}

	return caps
}

// inferFromDescription extracts capability keywords from a description.
func inferFromDescription(description string) []string {
	var caps []string
	descLower := strings.ToLower(description)

	// Capability keywords with their tags
	keywords := map[string]string{
		"read":    "read",
		"write":   "write",
		"create":  "create",
		"delete":  "delete",
		"modify":  "modify",
		"execute": "execute",
		"run":     "execute",
		"search":  "search",
		"file":    "filesystem",
		"folder": "filesystem",
		"directory": "filesystem",
		"web": "web",
		"http": "http",
		"api": "api",
		"database": "database",
		"sql": "database",
		"shell": "shell",
		"command": "command",
		"code": "code",
		"git": "vcs",
		"github": "vcs",
	}

	capMap := make(map[string]bool)
	for keyword, capability := range keywords {
		if strings.Contains(descLower, keyword) {
			capMap[capability] = true
		}
	}

	for cap := range capMap {
		caps = append(caps, cap)
	}

	return caps
}
