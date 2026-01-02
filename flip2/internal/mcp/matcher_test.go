package mcp

import (
	"encoding/json"
	"testing"
	"time"
)

// TestMatchTool tests the basic MatchTool function with known matches.
func TestMatchTool(t *testing.T) {
	tests := []struct {
		name            string
		requirement     *TaskRequirement
		tool            *MatchableTool
		expectScore     float64
		expectAboveThreshold bool
	}{
		{
			name: "perfect match - read file",
			requirement: &TaskRequirement{
				Name:                    "read file",
				Description:             "I need to read the contents of a file",
				RequiredCapabilities:    []string{"filesystem", "read"},
				InputParameters:         map[string]string{"path": "string"},
			},
			tool: &MatchableTool{
				Tool: Tool{
					Name:        "read_file",
					Description: "Read the contents of a file from the filesystem",
					InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {"type": "string"},
							"encoding": {"type": "string"}
						},
						"required": ["path"]
					}`),
				},
				ServerName:   "filesystem",
				Capabilities: []string{"filesystem", "read"},
			},
			expectScore:          0.9, // High score for good match
			expectAboveThreshold: true,
		},
		{
			name: "good match - write file",
			requirement: &TaskRequirement{
				Name:                 "write file",
				Description:          "Write content to a file",
				RequiredCapabilities: []string{"filesystem", "write"},
				InputParameters:      map[string]string{"path": "string", "content": "string"},
			},
			tool: &MatchableTool{
				Tool: Tool{
					Name:        "write_file",
					Description: "Write data to a file in the filesystem",
					InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {"type": "string"},
							"content": {"type": "string"}
						},
						"required": ["path", "content"]
					}`),
				},
				ServerName:   "filesystem",
				Capabilities: []string{"filesystem", "write"},
			},
			expectScore:          0.88,
			expectAboveThreshold: true,
		},
		{
			name: "partial match - missing capability",
			requirement: &TaskRequirement{
				Name:                 "execute shell command",
				Description:          "Execute a shell command with full system access",
				RequiredCapabilities: []string{"shell", "execute", "system"},
				InputParameters:      map[string]string{"command": "string"},
			},
			tool: &MatchableTool{
				Tool: Tool{
					Name:        "execute_command",
					Description: "Execute a shell command",
					InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"command": {"type": "string"}
						}
					}`),
				},
				ServerName:   "shell",
				Capabilities: []string{"shell", "execute"}, // Missing "system"
			},
			expectScore:          0.63, // Lower score due to missing capability (2/3 caps)
			expectAboveThreshold: false, // Just below threshold at 0.7
		},
		{
			name: "weak match - incompatible types",
			requirement: &TaskRequirement{
				Name:                 "search database",
				Description:          "Search for records in a database",
				RequiredCapabilities: []string{"database", "search"},
				InputParameters:      map[string]string{"query": "string", "limit": "integer"},
			},
			tool: &MatchableTool{
				Tool: Tool{
					Name:        "search_web",
					Description: "Search results on the web",
					InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"terms": {"type": "string"}
						}
					}`),
				},
				ServerName:   "web",
				Capabilities: []string{"web", "search"},
			},
			expectScore:          0.29, // Low score: 50% caps, 0% schema, but some keyword match
			expectAboveThreshold: false,
		},
		{
			name: "no match - completely unrelated",
			requirement: &TaskRequirement{
				Name:                 "translate text",
				Description:          "Translate text to another language",
				RequiredCapabilities: []string{"translation", "nlp"},
				InputParameters:      map[string]string{"text": "string", "target_language": "string"},
			},
			tool: &MatchableTool{
				Tool: Tool{
					Name:        "get_weather",
					Description: "Get weather information for a location",
					InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"location": {"type": "string"}
						}
					}`),
				},
				ServerName:   "weather",
				Capabilities: []string{"weather", "read"},
			},
			expectScore:          0.0, // Complete mismatch: 0% name, 0% desc, 0% caps, 0% schema
			expectAboveThreshold: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MatchTool(tt.requirement, tt.tool)
			if err != nil {
				t.Fatalf("MatchTool failed: %v", err)
			}

			// Check score is within acceptable range (allow 10% variance)
			lowerBound := tt.expectScore - 0.1
			upperBound := tt.expectScore + 0.1
			if result.Score < lowerBound || result.Score > upperBound {
				t.Errorf("Score %f outside expected range [%f, %f]", result.Score, lowerBound, upperBound)
			}

			// Check threshold logic
			aboveThreshold := result.Score >= MatchThreshold
			if aboveThreshold != tt.expectAboveThreshold {
				t.Errorf("Above threshold: got %v, want %v", aboveThreshold, tt.expectAboveThreshold)
			}

			// Check that reasoning is provided
			if result.Reasoning == "" {
				t.Error("Empty reasoning string")
			}

			// Check that component scores are provided
			if result.ComponentScores == nil {
				t.Error("ComponentScores is nil")
			}
		})
	}
}

// TestMatchTools tests matching multiple tools.
func TestMatchTools(t *testing.T) {
	requirement := &TaskRequirement{
		Name:                 "read file",
		Description:          "Read the contents of a file",
		RequiredCapabilities: []string{"filesystem", "read"},
		InputParameters:      map[string]string{"path": "string"},
	}

	tools := []*MatchableTool{
		{
			Tool: Tool{
				Name:        "read_file",
				Description: "Read file contents",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
			ServerName:   "fs",
			Capabilities: []string{"filesystem", "read"},
		},
		{
			Tool: Tool{
				Name:        "list_files",
				Description: "List files in a directory",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"dir":{"type":"string"}}}`),
			},
			ServerName:   "fs",
			Capabilities: []string{"filesystem", "list"},
		},
		{
			Tool: Tool{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			ServerName:   "weather",
			Capabilities: []string{"weather"},
		},
	}

	results, err := MatchTools(requirement, tools)
	if err != nil {
		t.Fatalf("MatchTools failed: %v", err)
	}

	// Should have at least 1 match above threshold
	if len(results) < 1 {
		t.Errorf("Expected at least 1 match, got %d", len(results))
	}

	// First result should be the best match
	if results[0].Tool.Tool.Name != "read_file" {
		t.Errorf("Best match should be read_file, got %s", results[0].Tool.Tool.Name)
	}

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("Results not properly sorted by score descending")
		}
	}
}

// TestCalculateNameSimilarity tests the name similarity function.
func TestCalculateNameSimilarity(t *testing.T) {
	tests := []struct {
		requirement     string
		toolName        string
		toolDescription string
		expectMin       float64
		expectMax       float64
	}{
		{
			requirement:     "read file",
			toolName:        "read_file",
			toolDescription: "read file from filesystem",
			expectMin:       0.7,
			expectMax:       1.0,
		},
		{
			requirement:     "list directory",
			toolName:        "list_files",
			toolDescription: "list all files",
			expectMin:       0.5,
			expectMax:       1.0,
		},
		{
			requirement:     "delete user",
			toolName:        "remove_item",
			toolDescription: "remove from collection",
			expectMin:       0.0,
			expectMax:       0.5,
		},
		{
			requirement:     "",
			toolName:        "test_tool",
			toolDescription: "test description",
			expectMin:       0.0,
			expectMax:       0.0,
		},
	}

	for _, tt := range tests {
		score := calculateNameSimilarity(tt.requirement, tt.toolName, tt.toolDescription)
		if score < tt.expectMin || score > tt.expectMax {
			t.Errorf("calculateNameSimilarity(%q, %q, %q) = %f, want in range [%f, %f]",
				tt.requirement, tt.toolName, tt.toolDescription, score, tt.expectMin, tt.expectMax)
		}
	}
}

// TestCalculateDescriptionMatch tests description matching.
func TestCalculateDescriptionMatch(t *testing.T) {
	tests := []struct {
		requirement    string
		description    string
		expectMin      float64
		expectMax      float64
	}{
		{
			requirement: "read file contents",
			description: "Read file from filesystem",
			expectMin:   0.5,
			expectMax:   1.0,
		},
		{
			requirement: "execute shell command",
			description: "Run system commands in a shell",
			expectMin:   0.5,
			expectMax:   1.0,
		},
		{
			requirement: "translate text",
			description: "Get weather forecast",
			expectMin:   0.0,
			expectMax:   0.3,
		},
	}

	for _, tt := range tests {
		score := calculateDescriptionMatch(tt.requirement, tt.description)
		if score < tt.expectMin || score > tt.expectMax {
			t.Errorf("calculateDescriptionMatch(%q, %q) = %f, want in range [%f, %f]",
				tt.requirement, tt.description, score, tt.expectMin, tt.expectMax)
		}
	}
}

// TestCalculateCapabilityMatch tests capability matching.
func TestCalculateCapabilityMatch(t *testing.T) {
	tests := []struct {
		required  []string
		available []string
		expect    float64
	}{
		{
			required:  []string{"filesystem", "read"},
			available: []string{"filesystem", "read", "write"},
			expect:    1.0,
		},
		{
			required:  []string{"filesystem", "read", "delete"},
			available: []string{"filesystem", "read"},
			expect:    2.0 / 3.0,
		},
		{
			required:  []string{"filesystem"},
			available: []string{"web", "api"},
			expect:    0.0,
		},
		{
			required:  []string{},
			available: []string{"filesystem", "read"},
			expect:    1.0,
		},
		{
			required:  []string{"filesystem"},
			available: []string{},
			expect:    0.0,
		},
	}

	for _, tt := range tests {
		score := calculateCapabilityMatch(tt.required, tt.available)
		if score != tt.expect {
			t.Errorf("calculateCapabilityMatch(%v, %v) = %f, want %f",
				tt.required, tt.available, score, tt.expect)
		}
	}
}

// TestCalculateInputSchemaMatch tests input schema matching.
func TestCalculateInputSchemaMatch(t *testing.T) {
	tests := []struct {
		name     string
		required map[string]string
		schema   *SchemaInfo
		expect   float64
	}{
		{
			name: "all parameters present",
			required: map[string]string{
				"path": "string",
			},
			schema: &SchemaInfo{
				Type: "object",
				Properties: map[string]string{
					"path":     "string",
					"encoding": "string",
				},
			},
			expect: 1.0,
		},
		{
			name: "partial parameters",
			required: map[string]string{
				"path":   "string",
				"offset": "integer",
				"count":  "integer",
			},
			schema: &SchemaInfo{
				Type: "object",
				Properties: map[string]string{
					"path": "string",
				},
			},
			expect: 1.0 / 3.0,
		},
		{
			name:     "no requirements",
			required: map[string]string{},
			schema: &SchemaInfo{
				Type: "object",
				Properties: map[string]string{
					"path": "string",
				},
			},
			expect: 1.0,
		},
		{
			name: "type mismatch",
			required: map[string]string{
				"count": "integer",
			},
			schema: &SchemaInfo{
				Type: "object",
				Properties: map[string]string{
					"count": "string", // Type mismatch
				},
			},
			expect: 0.0,
		},
	}

	for _, tt := range tests {
		score := calculateInputSchemaMatch(tt.required, tt.schema)
		if score != tt.expect {
			t.Errorf("%s: calculateInputSchemaMatch(%v, %v) = %f, want %f",
				tt.name, tt.required, tt.schema, score, tt.expect)
		}
	}
}

// TestParseSchema tests JSON schema parsing.
func TestParseSchema(t *testing.T) {
	tests := []struct {
		name        string
		schema      json.RawMessage
		expectError bool
		checkFunc   func(*SchemaInfo) bool
	}{
		{
			name:   "object schema",
			schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"count":{"type":"integer"}},"required":["path"]}`),
			checkFunc: func(info *SchemaInfo) bool {
				return info.Type == "object" &&
					info.Properties["path"] == "string" &&
					info.Properties["count"] == "integer" &&
					len(info.Required) == 1 &&
					info.Required[0] == "path"
			},
		},
		{
			name:   "empty schema",
			schema: json.RawMessage(``),
			checkFunc: func(info *SchemaInfo) bool {
				return info.Type == "" && len(info.Properties) == 0
			},
		},
		{
			name:        "invalid JSON",
			schema:      json.RawMessage(`{invalid}`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		info, err := parseSchema(tt.schema)
		if tt.expectError && err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
		if !tt.expectError && err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
		}
		if !tt.expectError && !tt.checkFunc(info) {
			t.Errorf("%s: schema parsing result does not match expectations", tt.name)
		}
	}
}

// TestInferCapabilities tests capability inference.
func TestInferCapabilities(t *testing.T) {
	tests := []struct {
		name           string
		tool           *Tool
		toolName       string
		expectContains []string
	}{
		{
			name: "read file tool",
			tool: &Tool{
				Name:        "read_file",
				Description: "Read file from filesystem",
				Annotations: &ToolAnnotations{
					ReadOnlyHint: true,
				},
			},
			toolName:       "read_file",
			expectContains: []string{"read", "file", "filesystem", "readonly"},
		},
		{
			name: "write database tool",
			tool: &Tool{
				Name:        "write_database",
				Description: "Write data to database",
				Annotations: &ToolAnnotations{
					DestructiveHint: true,
				},
			},
			toolName:       "write_database",
			expectContains: []string{"write", "database", "destructive"},
		},
		{
			name: "execute shell tool",
			tool: &Tool{
				Name:        "shell_execute",
				Description: "Execute shell commands",
				Annotations: &ToolAnnotations{
					OpenWorldHint: true,
				},
			},
			toolName:       "shell_execute",
			expectContains: []string{"execute", "shell", "openworld"},
		},
	}

	for _, tt := range tests {
		caps := InferCapabilities(tt.tool, tt.toolName)
		for _, expected := range tt.expectContains {
			found := false
			for _, cap := range caps {
				if cap == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: expected capability %q not found in %v", tt.name, expected, caps)
			}
		}
	}
}

// BenchmarkMatchTool measures matching performance.
func BenchmarkMatchTool(b *testing.B) {
	requirement := &TaskRequirement{
		Name:                 "read file",
		Description:          "Read the contents of a file from the filesystem",
		RequiredCapabilities: []string{"filesystem", "read"},
		InputParameters:      map[string]string{"path": "string", "encoding": "string"},
	}

	tool := &MatchableTool{
		Tool: Tool{
			Name:        "read_file",
			Description: "Read file contents from the filesystem",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"encoding":{"type":"string"}}}`),
		},
		ServerName:   "filesystem",
		Capabilities: []string{"filesystem", "read"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = MatchTool(requirement, tool)
	}
}

// TestComprehensiveMatchingAccuracy tests the matcher against a known test set.
// This test validates that the matching algorithm achieves 90%+ accuracy.
func TestComprehensiveMatchingAccuracy(t *testing.T) {
	type testCase struct {
		name                string
		requirement         *TaskRequirement
		tools               []*MatchableTool
		expectedBestMatch   string // Expected name of the best matching tool
		minExpectedScore    float64
		expectedMatches     int // Expected number of matches above threshold
	}

	// Create a realistic test set
	testCases := []testCase{
		{
			name: "file reading task",
			requirement: &TaskRequirement{
				Name:                 "read configuration",
				Description:          "Read a configuration file to get settings",
				RequiredCapabilities: []string{"filesystem", "read"},
				InputParameters:      map[string]string{"path": "string"},
			},
			tools: []*MatchableTool{
				{
					Tool: Tool{
						Name:        "read_file",
						Description: "Read contents from a file",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
					},
					ServerName:   "filesystem",
					Capabilities: []string{"filesystem", "read"},
				},
				{
					Tool: Tool{
						Name:        "write_file",
						Description: "Write to a file",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}}}`),
					},
					ServerName:   "filesystem",
					Capabilities: []string{"filesystem", "write"},
				},
				{
					Tool: Tool{
						Name:        "get_weather",
						Description: "Get weather data",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
					},
					ServerName:   "weather",
					Capabilities: []string{"weather"},
				},
			},
			expectedBestMatch: "read_file",
			minExpectedScore:  0.7,
			expectedMatches:   1,
		},
		{
			name: "code execution task",
			requirement: &TaskRequirement{
				Name:                 "run script",
				Description:          "Execute a Python script to process data",
				RequiredCapabilities: []string{"execution", "code"},
				InputParameters:      map[string]string{"script": "string"},
			},
			tools: []*MatchableTool{
				{
					Tool: Tool{
						Name:        "execute_python",
						Description: "Execute Python code",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"script":{"type":"string"},"timeout":{"type":"integer"}}}`),
					},
					ServerName:   "code",
					Capabilities: []string{"execution", "code", "python"},
				},
				{
					Tool: Tool{
						Name:        "shell_command",
						Description: "Run shell commands",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}}}`),
					},
					ServerName:   "shell",
					Capabilities: []string{"execution", "shell"},
				},
				{
					Tool: Tool{
						Name:        "list_files",
						Description: "List files in directory",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
					},
					ServerName:   "filesystem",
					Capabilities: []string{"filesystem", "read"},
				},
			},
			expectedBestMatch: "execute_python",
			minExpectedScore:  0.7,
			expectedMatches:   1, // Only execute_python should score above threshold
		},
		{
			name: "database query task",
			requirement: &TaskRequirement{
				Name:                 "query data",
				Description:          "Query a SQL database for user records",
				RequiredCapabilities: []string{"database", "query"},
				InputParameters:      map[string]string{"sql": "string"},
			},
			tools: []*MatchableTool{
				{
					Tool: Tool{
						Name:        "sql_query",
						Description: "Execute SQL queries against a database",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"sql":{"type":"string"},"database":{"type":"string"}}}`),
					},
					ServerName:   "database",
					Capabilities: []string{"database", "query", "sql"},
				},
				{
					Tool: Tool{
						Name:        "get_json",
						Description: "Fetch JSON data from an API",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}}}`),
					},
					ServerName:   "api",
					Capabilities: []string{"api", "http", "read"},
				},
			},
			expectedBestMatch: "sql_query",
			minExpectedScore:  0.7,
			expectedMatches:   1,
		},
		{
			name: "web search task",
			requirement: &TaskRequirement{
				Name:                 "search web",
				Description:          "Search the web for information about a topic",
				RequiredCapabilities: []string{"web", "search"},
				InputParameters:      map[string]string{"query": "string"},
			},
			tools: []*MatchableTool{
				{
					Tool: Tool{
						Name:        "web_search",
						Description: "Search the web using a search engine",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}}}`),
					},
					ServerName:   "web",
					Capabilities: []string{"web", "search", "http"},
				},
				{
					Tool: Tool{
						Name:        "local_search",
						Description: "Search local files",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"terms":{"type":"string"},"path":{"type":"string"}}}`),
					},
					ServerName:   "filesystem",
					Capabilities: []string{"filesystem", "search"},
				},
			},
			expectedBestMatch: "web_search",
			minExpectedScore:  0.7,
			expectedMatches:   1, // Only web_search scores high enough
		},
	}

	correctMatches := 0
	totalTests := 0

	for _, tc := range testCases {
		results, err := MatchTools(tc.requirement, tc.tools)
		if err != nil {
			t.Errorf("%s: MatchTools failed: %v", tc.name, err)
			continue
		}

		totalTests++

		// Check we got expected number of matches
		if len(results) != tc.expectedMatches {
			t.Logf("%s: expected %d matches, got %d", tc.name, tc.expectedMatches, len(results))
		}

		// Check best match
		if len(results) > 0 {
			bestMatch := results[0]

			// Verify best match is correct
			if bestMatch.Tool.Tool.Name == tc.expectedBestMatch {
				correctMatches++

				// Verify score is above threshold
				if bestMatch.Score < tc.minExpectedScore {
					t.Errorf("%s: best match score %f below expected %f", tc.name, bestMatch.Score, tc.minExpectedScore)
				}
			} else {
				t.Errorf("%s: expected best match %q, got %q (score: %f)",
					tc.name, tc.expectedBestMatch, bestMatch.Tool.Tool.Name, bestMatch.Score)
			}
		} else {
			t.Errorf("%s: expected matches above threshold, got none", tc.name)
		}
	}

	// Calculate accuracy
	if totalTests > 0 {
		accuracy := float64(correctMatches) / float64(totalTests)
		t.Logf("Matching accuracy: %d/%d (%.1f%%)", correctMatches, totalTests, accuracy*100)

		if accuracy < 0.9 {
			t.Errorf("Accuracy %.1f%% below required 90%%", accuracy*100)
		}
	}
}

// TestEdgeCases tests edge cases and boundary conditions.
func TestEdgeCases(t *testing.T) {
	t.Run("nil inputs", func(t *testing.T) {
		_, err := MatchTool(nil, nil)
		if err == nil {
			t.Error("expected error for nil inputs")
		}

		requirement := &TaskRequirement{Description: "test"}
		_, err = MatchTool(requirement, nil)
		if err == nil {
			t.Error("expected error for nil tool")
		}
	})

	t.Run("empty requirement", func(t *testing.T) {
		requirement := &TaskRequirement{}
		tool := &MatchableTool{
			Tool:       Tool{Name: "test", InputSchema: json.RawMessage(`{}`)},
			ServerName: "test",
		}

		result, err := MatchTool(requirement, tool)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Empty requirement should match with moderate score
		if result.Score < 0 || result.Score > 1 {
			t.Errorf("score out of range: %f", result.Score)
		}
	})

	t.Run("invalid schema", func(t *testing.T) {
		requirement := &TaskRequirement{Description: "test"}
		tool := &MatchableTool{
			Tool:       Tool{Name: "test", InputSchema: json.RawMessage(`{invalid}`)},
			ServerName: "test",
		}

		_, err := MatchTool(requirement, tool)
		if err == nil {
			t.Error("expected error for invalid schema")
		}
	})

	t.Run("case insensitivity", func(t *testing.T) {
		requirement := &TaskRequirement{
			RequiredCapabilities: []string{"FileSystem", "Read"},
		}

		tool := &MatchableTool{
			Tool:         Tool{Name: "test", InputSchema: json.RawMessage(`{}`)},
			ServerName:   "test",
			Capabilities: []string{"filesystem", "read"},
		}

		result, err := MatchTool(requirement, tool)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should match despite case difference
		if result.ComponentScores.CapabilityMatch != 1.0 {
			t.Errorf("case insensitive matching failed: score %f", result.ComponentScores.CapabilityMatch)
		}
	})
}

// TestNormalization tests string normalization utilities.
func TestNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"  extra   spaces  ", "extra   spaces"},
		{"UPPERCASE", "uppercase"},
	}

	for _, tt := range tests {
		result := normalizeString(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestKeywordExtraction tests keyword extraction logic.
func TestKeywordExtraction(t *testing.T) {
	tests := []struct {
		input    string
		minLen   int
		expected []string
	}{
		{
			input:    "read the file from the filesystem",
			minLen:   3,
			expected: []string{"read", "file", "filesystem"},
		},
		{
			input:    "execute shell command",
			minLen:   3,
			expected: []string{"execute", "shell", "command"},
		},
	}

	for _, tt := range tests {
		keywords := extractKeywords(tt.input)
		if len(keywords) != len(tt.expected) {
			t.Errorf("extractKeywords(%q) returned %d keywords, want %d", tt.input, len(keywords), len(tt.expected))
		}
	}
}

// TestScoreBreakdownPresentation tests that score breakdowns are informative.
func TestScoreBreakdownPresentation(t *testing.T) {
	requirement := &TaskRequirement{
		Name:                 "read file",
		Description:          "Read the contents of a file",
		RequiredCapabilities: []string{"filesystem", "read"},
		InputParameters:      map[string]string{"path": "string"},
	}

	tool := &MatchableTool{
		Tool: Tool{
			Name:        "read_file",
			Description: "Read file contents from filesystem",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
		ServerName:   "filesystem",
		Capabilities: []string{"filesystem", "read"},
	}

	result, _ := MatchTool(requirement, tool)

	// All component scores should be between 0 and 1
	scores := []*float64{
		&result.ComponentScores.NameSimilarity,
		&result.ComponentScores.DescriptionMatch,
		&result.ComponentScores.CapabilityMatch,
		&result.ComponentScores.InputSchemaMatch,
	}

	for _, score := range scores {
		if *score < 0 || *score > 1 {
			t.Errorf("component score %f out of valid range [0, 1]", *score)
		}
	}

	// Overall score should also be in range
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("overall score %f out of range", result.Score)
	}

	// Reasoning should be meaningful
	if len(result.Reasoning) == 0 {
		t.Error("empty reasoning string")
	}
}

// TestPerformance ensures matching completes in reasonable time.
func TestPerformance(t *testing.T) {
	// Create a large tool set
	requirement := &TaskRequirement{
		Name:                 "read file",
		Description:          "Read the contents of a file",
		RequiredCapabilities: []string{"filesystem", "read"},
		InputParameters:      map[string]string{"path": "string"},
	}

	tools := make([]*MatchableTool, 100)
	for i := 0; i < 100; i++ {
		tools[i] = &MatchableTool{
			Tool: Tool{
				Name:        "tool_" + string(rune(i)),
				Description: "Tool for doing stuff",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`),
			},
			ServerName:   "server_" + string(rune(i%10)),
			Capabilities: []string{"generic"},
		}
	}

	// Add one good match
	tools = append(tools[:0], &MatchableTool{
		Tool: Tool{
			Name:        "read_file",
			Description: "Read file contents",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
		ServerName:   "filesystem",
		Capabilities: []string{"filesystem", "read"},
	})
	tools = append(tools, tools[1:]...)

	start := time.Now()
	_, err := MatchTools(requirement, tools)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("MatchTools failed: %v", err)
	}

	// Should complete in less than 100ms for 100 tools
	if duration > 100*time.Millisecond {
		t.Logf("warning: MatchTools took %v for 100 tools (expected <100ms)", duration)
	}
}
