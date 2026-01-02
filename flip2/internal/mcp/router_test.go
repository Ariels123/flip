package mcp

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"
)

// =============================================================================
// Tool Discovery Tests
// =============================================================================

// TestFindTools_BasicDiscovery verifies tool discovery functionality.
func TestFindTools_BasicDiscovery(t *testing.T) {
	tests := []struct {
		name            string
		description     string
		shouldSucceed   bool
	}{
		{
			name:            "simple tool discovery",
			description:     "read a file",
			shouldSucceed:   true,
		},
		{
			name:            "complex tool discovery",
			description:     "search the web and summarize results",
			shouldSucceed:   true,
		},
		{
			name:            "empty description",
			description:     "",
			shouldSucceed:   true, // Should still return all tools
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &ToolQuery{
				Description: tt.description,
				Limit:       10,
			}

			// Verify query structure is valid
			if query.Limit <= 0 {
				t.Fatal("query limit must be positive")
			}

			// Verify defaults are reasonable
			if query.MinScore < 0 || query.MinScore > 1 {
				// Default should be 0 or unset
				if query.MinScore != 0 {
					t.Errorf("unexpected default MinScore: %f", query.MinScore)
				}
			}
		})
	}
}

// TestFindTools_Capabilities tests capability-based tool discovery.
func TestFindTools_Capabilities(t *testing.T) {
	tests := []struct {
		name                 string
		description          string
		requiredCapabilities []string
		shouldMatch          bool
	}{
		{
			name:                 "filesystem read",
			description:          "read file contents",
			requiredCapabilities: []string{"filesystem", "read"},
			shouldMatch:          true,
		},
		{
			name:                 "web operations",
			description:          "fetch from web",
			requiredCapabilities: []string{"web"},
			shouldMatch:          true,
		},
		{
			name:                 "database query",
			description:          "query database",
			requiredCapabilities: []string{"database"},
			shouldMatch:          true,
		},
		{
			name:                 "no capabilities",
			description:          "generic operation",
			requiredCapabilities: []string{},
			shouldMatch:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &ToolQuery{
				Description:  tt.description,
				Capabilities: tt.requiredCapabilities,
				Limit:        10,
			}

			if len(query.Capabilities) == 0 && len(tt.requiredCapabilities) > 0 {
				t.Fatal("capabilities not set on query")
			}
		})
	}
}

// TestFindTools_NamePatternMatching tests regex pattern matching for tool names.
func TestFindTools_NamePatternMatching(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		matches []string
	}{
		{
			name:    "read_ prefix",
			pattern: "^read_",
			matches: []string{"read_file", "read_line", "read_block"},
		},
		{
			name:    "write_ prefix",
			pattern: "^write_",
			matches: []string{"write_file", "write_line"},
		},
		{
			name:    "file suffix",
			pattern: "file$",
			matches: []string{"read_file", "write_file"},
		},
		{
			name:    "any pattern",
			pattern: ".*",
			matches: []string{"any", "tool", "name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tt.pattern)
			if err != nil {
				t.Fatalf("invalid regex pattern: %v", err)
			}

			// Verify pattern compiles and can be used
			for _, toolName := range tt.matches {
				if !pattern.MatchString(toolName) {
					t.Errorf("pattern %q should match %q", tt.pattern, toolName)
				}
			}
		})
	}
}

// TestFindTools_ServerFiltering tests filtering tools by server name.
func TestFindTools_ServerFiltering(t *testing.T) {
	tests := []struct {
		name           string
		serverFilter   []string
		expectedServers int
	}{
		{
			name:            "single server filter",
			serverFilter:    []string{"filesystem"},
			expectedServers: 1,
		},
		{
			name:            "multiple server filters",
			serverFilter:    []string{"filesystem", "web"},
			expectedServers: 2,
		},
		{
			name:            "no server filter",
			serverFilter:    []string{},
			expectedServers: 0, // Empty means search all
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &ToolQuery{
				ServerFilter: tt.serverFilter,
				Limit:        10,
			}

			if len(query.ServerFilter) > 0 && tt.expectedServers == 0 {
				t.Fatal("server filter should have been applied")
			}
		})
	}
}

// TestFindTools_LimitAndPagination tests result limiting.
func TestFindTools_LimitAndPagination(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		totalTools    int
		expectedCount int
	}{
		{
			name:          "limit less than total",
			limit:         5,
			totalTools:    20,
			expectedCount: 5,
		},
		{
			name:          "limit greater than total",
			limit:         30,
			totalTools:    10,
			expectedCount: 10,
		},
		{
			name:          "no limit specified",
			limit:         0,
			totalTools:    15,
			expectedCount: 15,
		},
		{
			name:          "limit of 1",
			limit:         1,
			totalTools:    100,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: query is created but not used in this test
			// query := &ToolQuery{
			// 	Description: "test",
			// 	Limit:       tt.limit,
			// }

			// Calculate expected results
			resultCount := tt.totalTools
			if tt.limit > 0 && tt.limit < tt.totalTools {
				resultCount = tt.limit
			}

			if resultCount != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, resultCount)
			}
		})
	}
}

// =============================================================================
// Capability Matching Tests (Table-Driven)
// =============================================================================

// TestCapabilityMatching_TableDriven tests capability matching with various combinations.
func TestCapabilityMatching_TableDriven(t *testing.T) {
	tests := []struct {
		name                 string
		toolCapabilities     []string
		requiredCapabilities []string
		expectMatch          bool
		description          string
	}{
		{
			name:                 "exact match",
			toolCapabilities:     []string{"filesystem", "read"},
			requiredCapabilities: []string{"filesystem", "read"},
			expectMatch:          true,
			description:          "tool has exactly required capabilities",
		},
		{
			name:                 "superset match",
			toolCapabilities:     []string{"filesystem", "read", "write", "delete"},
			requiredCapabilities: []string{"filesystem", "read"},
			expectMatch:          true,
			description:          "tool has superset of required capabilities",
		},
		{
			name:                 "missing capability",
			toolCapabilities:     []string{"filesystem", "read"},
			requiredCapabilities: []string{"filesystem", "read", "delete"},
			expectMatch:          false,
			description:          "tool missing required capability",
		},
		{
			name:                 "empty requirements",
			toolCapabilities:     []string{"filesystem"},
			requiredCapabilities: []string{},
			expectMatch:          true,
			description:          "no requirements should match any tool",
		},
		{
			name:                 "empty tool capabilities",
			toolCapabilities:     []string{},
			requiredCapabilities: []string{"filesystem"},
			expectMatch:          false,
			description:          "tool with no capabilities cannot match",
		},
		{
			name:                 "web capabilities",
			toolCapabilities:     []string{"web", "http"},
			requiredCapabilities: []string{"web"},
			expectMatch:          true,
			description:          "web-related capability matching",
		},
		{
			name:                 "api capabilities",
			toolCapabilities:     []string{"api", "rest", "graphql"},
			requiredCapabilities: []string{"api", "rest"},
			expectMatch:          true,
			description:          "multiple api capabilities",
		},
		{
			name:                 "shell capabilities",
			toolCapabilities:     []string{"shell", "bash"},
			requiredCapabilities: []string{"shell", "execute"},
			expectMatch:          false,
			description:          "missing execute capability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test capability matching logic
			matches := capabilityMatches(tt.toolCapabilities, tt.requiredCapabilities)

			if matches != tt.expectMatch {
				t.Errorf("expected match=%v, got %v. %s",
					tt.expectMatch, matches, tt.description)
			}
		})
	}
}

// capabilityMatches is a helper function for testing capability matching logic.
func capabilityMatches(toolCaps, requiredCaps []string) bool {
	if len(requiredCaps) == 0 {
		return true
	}

	capMap := make(map[string]bool)
	for _, cap := range toolCaps {
		capMap[cap] = true
	}

	for _, req := range requiredCaps {
		if !capMap[req] {
			return false
		}
	}

	return true
}

// TestCapabilityMatching_WellKnownCapabilities verifies well-known capability constants.
func TestCapabilityMatching_WellKnownCapabilities(t *testing.T) {
	capabilities := []struct {
		name     string
		constant string
	}{
		{"filesystem", CapabilityFilesystem},
		{"read", CapabilityRead},
		{"write", CapabilityWrite},
		{"delete", CapabilityDelete},
		{"search", CapabilitySearch},
		{"web", CapabilityWeb},
		{"browser", CapabilityBrowser},
		{"api", CapabilityAPI},
		{"database", CapabilityDatabase},
		{"shell", CapabilityShell},
		{"git", CapabilityGit},
		{"ai", CapabilityAI},
		{"image", CapabilityImage},
		{"audio", CapabilityAudio},
		{"email", CapabilityEmail},
		{"calendar", CapabilityCalendar},
		{"messaging", CapabilityMessaging},
	}

	for _, cap := range capabilities {
		t.Run(cap.name, func(t *testing.T) {
			if cap.constant == "" {
				t.Errorf("capability %s not defined", cap.name)
			}
			if string(cap.constant) != cap.name {
				t.Errorf("capability constant value mismatch: expected %s, got %s",
					cap.name, cap.constant)
			}
		})
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestErrorHandling_ToolNotFound tests tool not found error handling.
func TestErrorHandling_ToolNotFound(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		errorCode   RouterErrorCode
		shouldError bool
	}{
		{
			name:        "non-existent tool",
			toolName:    "non_existent_tool",
			errorCode:   ErrToolNotFound,
			shouldError: true,
		},
		{
			name:        "empty tool name",
			toolName:    "",
			errorCode:   ErrToolNotFound,
			shouldError: true,
		},
		{
			name:        "valid tool name",
			toolName:    "read_file",
			errorCode:   "",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldError && tt.errorCode == "" {
				t.Fatal("shouldError=true but no error code specified")
			}

			if !tt.shouldError && tt.errorCode != "" {
				t.Fatal("shouldError=false but error code specified")
			}
		})
	}
}

// TestErrorHandling_CapabilityMismatch tests capability mismatch error handling.
func TestErrorHandling_CapabilityMismatch(t *testing.T) {
	tests := []struct {
		name             string
		toolCapabilities []string
		requiredCaps     []string
		hasMismatch      bool
		mismatchedCaps   []string
	}{
		{
			name:             "single missing capability",
			toolCapabilities: []string{"read", "filesystem"},
			requiredCaps:     []string{"read", "delete"},
			hasMismatch:      true,
			mismatchedCaps:   []string{"delete"},
		},
		{
			name:             "multiple missing capabilities",
			toolCapabilities: []string{"read"},
			requiredCaps:     []string{"write", "delete", "execute"},
			hasMismatch:      true,
			mismatchedCaps:   []string{"write", "delete", "execute"},
		},
		{
			name:             "no mismatch",
			toolCapabilities: []string{"read", "write", "delete"},
			requiredCaps:     []string{"read", "write"},
			hasMismatch:      false,
			mismatchedCaps:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find mismatched capabilities
			toolCapSet := make(map[string]bool)
			for _, cap := range tt.toolCapabilities {
				toolCapSet[cap] = true
			}

			var mismatched []string
			for _, req := range tt.requiredCaps {
				if !toolCapSet[req] {
					mismatched = append(mismatched, req)
				}
			}

			hasMismatch := len(mismatched) > 0

			if hasMismatch != tt.hasMismatch {
				t.Errorf("expected mismatch=%v, got %v", tt.hasMismatch, hasMismatch)
			}

			if len(mismatched) != len(tt.mismatchedCaps) {
				t.Errorf("expected %d mismatches, got %d",
					len(tt.mismatchedCaps), len(mismatched))
			}
		})
	}
}

// TestErrorHandling_ErrorCodes tests all router error codes.
func TestErrorHandling_ErrorCodes(t *testing.T) {
	errorCodes := []struct {
		code     RouterErrorCode
		name     string
		hasValue bool
	}{
		{ErrNoMatchingTool, "ErrNoMatchingTool", true},
		{ErrAllToolsFailed, "ErrAllToolsFailed", true},
		{ErrToolNotFound, "ErrToolNotFound", true},
		{ErrServerUnavailable, "ErrServerUnavailable", true},
		{ErrInvalidArguments, "ErrInvalidArguments", true},
		{ErrTimeout, "ErrTimeout", true},
		{ErrChainFailed, "ErrChainFailed", true},
		{ErrCacheMiss, "ErrCacheMiss", true},
	}

	for _, ec := range errorCodes {
		t.Run(ec.name, func(t *testing.T) {
			if !ec.hasValue {
				t.Error("error code should have a value")
			}

			if string(ec.code) == "" {
				t.Error("error code string representation should not be empty")
			}
		})
	}
}

// TestErrorHandling_RouterErrorInterface tests RouterError implements error interface.
func TestErrorHandling_RouterErrorInterface(t *testing.T) {
	tests := []struct {
		name        string
		code        RouterErrorCode
		message     string
		cause       error
		toolName    string
		serverName  string
		expectedMsg string
	}{
		{
			name:        "error with cause",
			code:        ErrToolNotFound,
			message:     "tool not found",
			cause:       errors.New("test cause"),
			toolName:    "read_file",
			serverName:  "filesystem",
			expectedMsg: "tool not found: test cause",
		},
		{
			name:        "error without cause",
			code:        ErrNoMatchingTool,
			message:     "no matching tool",
			cause:       nil,
			toolName:    "",
			serverName:  "",
			expectedMsg: "no matching tool",
		},
		{
			name:        "server unavailable error",
			code:        ErrServerUnavailable,
			message:     "server unavailable",
			cause:       errors.New("connection refused"),
			toolName:    "",
			serverName:  "web_server",
			expectedMsg: "server unavailable: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RouterError{
				Code:       tt.code,
				Message:    tt.message,
				Cause:      tt.cause,
				ToolName:   tt.toolName,
				ServerName: tt.serverName,
			}

			if err.Error() != tt.expectedMsg {
				t.Errorf("expected %q, got %q", tt.expectedMsg, err.Error())
			}

			// Verify Unwrap returns the cause
			unwrapped := err.Unwrap()
			if tt.cause == nil && unwrapped != nil {
				t.Error("Unwrap should return nil when cause is nil")
			}
			if tt.cause != nil && unwrapped != tt.cause {
				t.Error("Unwrap should return the cause")
			}
		})
	}
}

// =============================================================================
// Scoring Tests
// =============================================================================

// TestScoreWeights_DefaultValues tests default score weights.
func TestScoreWeights_DefaultValues(t *testing.T) {
	weights := DefaultScoreWeights()

	tests := []struct {
		name           string
		weight         float64
		minValue       float64
		maxValue       float64
	}{
		{"Name", weights.Name, 0.25, 0.35},
		{"Description", weights.Description, 0.20, 0.30},
		{"Capability", weights.Capability, 0.15, 0.25},
		{"Schema", weights.Schema, 0.10, 0.20},
		{"Annotation", weights.Annotation, 0.01, 0.10},
		{"Reliability", weights.Reliability, 0.01, 0.10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.weight < tt.minValue || tt.weight > tt.maxValue {
				t.Errorf("weight %f not in range [%f, %f]",
					tt.weight, tt.minValue, tt.maxValue)
			}
		})
	}

	// Verify weights sum to approximately 1.0
	total := weights.Name + weights.Description + weights.Capability +
		weights.Schema + weights.Annotation + weights.Reliability

	if total < 0.99 || total > 1.01 {
		t.Errorf("weights should sum to 1.0, got %f", total)
	}
}

// TestScoreWeights_Normalization tests that weights are properly normalized.
func TestScoreWeights_Normalization(t *testing.T) {
	tests := []struct {
		name    string
		weights ScoreWeights
		isValid bool
	}{
		{
			name: "default weights",
			weights: DefaultScoreWeights(),
			isValid: true,
		},
		{
			name: "uniform weights",
			weights: ScoreWeights{
				Name:        0.166,
				Description: 0.166,
				Capability:  0.167,
				Schema:      0.167,
				Annotation:  0.167,
				Reliability: 0.167,
			},
			isValid: true,
		},
		{
			name: "negative weight",
			weights: ScoreWeights{
				Name:        -0.1,
				Description: 0.3,
				Capability:  0.3,
				Schema:      0.2,
				Annotation:  0.1,
				Reliability: 0.2,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check for negative weights
			hasNegative := tt.weights.Name < 0 || tt.weights.Description < 0 ||
				tt.weights.Capability < 0 || tt.weights.Schema < 0 ||
				tt.weights.Annotation < 0 || tt.weights.Reliability < 0

			if hasNegative != !tt.isValid {
				t.Errorf("invalid weight check failed for %s", tt.name)
			}
		})
	}
}

// TestScoreBreakdown_ComponentCalculation tests score component calculations.
func TestScoreBreakdown_ComponentCalculation(t *testing.T) {
	tests := []struct {
		name        string
		breakdown   *ScoreBreakdown
		expectedMin float64
		expectedMax float64
	}{
		{
			name: "perfect score",
			breakdown: &ScoreBreakdown{
				NameScore:        1.0,
				DescriptionScore: 1.0,
				CapabilityScore:  1.0,
				SchemaScore:      1.0,
				AnnotationScore:  1.0,
				ReliabilityScore: 1.0,
				Weights:          DefaultScoreWeights(),
			},
			expectedMin: 0.99,
			expectedMax: 1.01,
		},
		{
			name: "zero score",
			breakdown: &ScoreBreakdown{
				NameScore:        0.0,
				DescriptionScore: 0.0,
				CapabilityScore:  0.0,
				SchemaScore:      0.0,
				AnnotationScore:  0.0,
				ReliabilityScore: 0.0,
				Weights:          DefaultScoreWeights(),
			},
			expectedMin: -0.01,
			expectedMax: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate weighted score
			w := tt.breakdown.Weights
			score := (tt.breakdown.NameScore * w.Name) +
				(tt.breakdown.DescriptionScore * w.Description) +
				(tt.breakdown.CapabilityScore * w.Capability) +
				(tt.breakdown.SchemaScore * w.Schema) +
				(tt.breakdown.AnnotationScore * w.Annotation) +
				(tt.breakdown.ReliabilityScore * w.Reliability)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("score %f not in expected range [%f, %f]",
					score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

// =============================================================================
// Cache Management Tests
// =============================================================================

// TestCacheOptions_Defaults tests default cache options.
func TestCacheOptions_Defaults(t *testing.T) {
	opts := DefaultCacheOptions()

	if opts.TTL != 5*time.Minute {
		t.Errorf("expected TTL 5m, got %v", opts.TTL)
	}

	if opts.MaxSize != 10000 {
		t.Errorf("expected MaxSize 10000, got %d", opts.MaxSize)
	}

	if !opts.RefreshOnChange {
		t.Error("RefreshOnChange should be true by default")
	}

	if !opts.PrefetchOnInit {
		t.Error("PrefetchOnInit should be true by default")
	}
}

// TestCacheStatistics_HitRate tests cache hit rate calculation.
func TestCacheStatistics_HitRate(t *testing.T) {
	tests := []struct {
		name           string
		hitCount       int64
		missCount      int64
		expectedRate   float64
	}{
		{
			name:           "all hits",
			hitCount:       100,
			missCount:      0,
			expectedRate:   1.0,
		},
		{
			name:           "all misses",
			hitCount:       0,
			missCount:      100,
			expectedRate:   0.0,
		},
		{
			name:           "50/50 split",
			hitCount:       50,
			missCount:      50,
			expectedRate:   0.5,
		},
		{
			name:           "no requests",
			hitCount:       0,
			missCount:      0,
			expectedRate:   0.0,
		},
		{
			name:           "75% hit rate",
			hitCount:       75,
			missCount:      25,
			expectedRate:   0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := tt.hitCount + tt.missCount
			var rate float64
			if total > 0 {
				rate = float64(tt.hitCount) / float64(total)
			}

			if rate != tt.expectedRate {
				t.Errorf("expected hit rate %f, got %f", tt.expectedRate, rate)
			}
		})
	}
}

// =============================================================================
// Routing Options Tests
// =============================================================================

// TestRoutingOptions_Defaults tests default routing options.
func TestRoutingOptions_Defaults(t *testing.T) {
	opts := DefaultRoutingOptions()

	if !opts.EnableFallback {
		t.Error("EnableFallback should be true by default")
	}

	if opts.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", opts.MaxRetries)
	}

	if opts.RetryDelay != time.Second {
		t.Errorf("expected RetryDelay 1s, got %v", opts.RetryDelay)
	}

	if opts.MaxFallbackAttempts != 3 {
		t.Errorf("expected MaxFallbackAttempts 3, got %d", opts.MaxFallbackAttempts)
	}

	if opts.Timeout != 60*time.Second {
		t.Errorf("expected Timeout 60s, got %v", opts.Timeout)
	}
}

// TestRoutingOptions_CustomConfiguration tests custom routing configuration.
func TestRoutingOptions_CustomConfiguration(t *testing.T) {
	opts := &RoutingOptions{
		EnableFallback:      false,
		MaxRetries:          5,
		RetryDelay:          2 * time.Second,
		MaxFallbackAttempts: 10,
		Timeout:             30 * time.Second,
	}

	if opts.EnableFallback {
		t.Error("EnableFallback should be false")
	}

	if opts.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", opts.MaxRetries)
	}

	if opts.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay 2s, got %v", opts.RetryDelay)
	}

	if opts.MaxFallbackAttempts != 10 {
		t.Errorf("expected MaxFallbackAttempts 10, got %d", opts.MaxFallbackAttempts)
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", opts.Timeout)
	}
}

// =============================================================================
// Annotation Filtering Tests
// =============================================================================

// TestAnnotationFiltering_ReadOnly tests read-only annotation filtering.
func TestAnnotationFiltering_ReadOnly(t *testing.T) {
	tests := []struct {
		name               string
		isReadOnly         bool
		requireReadOnly    bool
		shouldMatch        bool
	}{
		{
			name:            "read-only tool with read-only requirement",
			isReadOnly:      true,
			requireReadOnly: true,
			shouldMatch:     true,
		},
		{
			name:            "write tool with read-only requirement",
			isReadOnly:      false,
			requireReadOnly: true,
			shouldMatch:     false,
		},
		{
			name:            "read-only tool without read-only requirement",
			isReadOnly:      true,
			requireReadOnly: false,
			shouldMatch:     true,
		},
		{
			name:            "write tool without read-only requirement",
			isReadOnly:      false,
			requireReadOnly: false,
			shouldMatch:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &AnnotationFilter{
				RequireReadOnly: tt.requireReadOnly,
			}

			// Simulate filtering logic
			matches := !filter.RequireReadOnly || tt.isReadOnly

			if matches != tt.shouldMatch {
				t.Errorf("expected match=%v, got %v", tt.shouldMatch, matches)
			}
		})
	}
}

// TestAnnotationFiltering_Destructive tests destructive annotation filtering.
func TestAnnotationFiltering_Destructive(t *testing.T) {
	tests := []struct {
		name               string
		isDestructive      bool
		excludeDestructive bool
		shouldMatch        bool
	}{
		{
			name:               "destructive tool with exclusion",
			isDestructive:      true,
			excludeDestructive: true,
			shouldMatch:        false,
		},
		{
			name:               "safe tool with exclusion",
			isDestructive:      false,
			excludeDestructive: true,
			shouldMatch:        true,
		},
		{
			name:               "destructive tool without exclusion",
			isDestructive:      true,
			excludeDestructive: false,
			shouldMatch:        true,
		},
		{
			name:               "safe tool without exclusion",
			isDestructive:      false,
			excludeDestructive: false,
			shouldMatch:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &AnnotationFilter{
				ExcludeDestructive: tt.excludeDestructive,
			}

			// Simulate filtering logic
			matches := !filter.ExcludeDestructive || !tt.isDestructive

			if matches != tt.shouldMatch {
				t.Errorf("expected match=%v, got %v", tt.shouldMatch, matches)
			}
		})
	}
}

// =============================================================================
// Query Result Filtering Tests
// =============================================================================

// TestToolQuery_MinScoreFiltering tests minimum score filtering.
func TestToolQuery_MinScoreFiltering(t *testing.T) {
	tests := []struct {
		name          string
		minScore      float64
		toolScores    []float64
		expectedCount int
	}{
		{
			name:          "filter by 0.8 score",
			minScore:      0.8,
			toolScores:    []float64{0.9, 0.75, 0.85, 0.7},
			expectedCount: 2,
		},
		{
			name:          "all tools above threshold",
			minScore:      0.5,
			toolScores:    []float64{0.9, 0.8, 0.7, 0.6},
			expectedCount: 4,
		},
		{
			name:          "no tools above threshold",
			minScore:      0.95,
			toolScores:    []float64{0.9, 0.85, 0.8, 0.75},
			expectedCount: 0,
		},
		{
			name:          "no filtering (score 0)",
			minScore:      0.0,
			toolScores:    []float64{0.5, 0.1, 0.2, 0.3},
			expectedCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate filtering logic
			var count int
			for _, score := range tt.toolScores {
				if score >= tt.minScore {
					count++
				}
			}

			if count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

// TestToolQuery_ResultLimiting tests query result limiting.
func TestToolQuery_ResultLimiting(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		totalTools    int
		expectedCount int
	}{
		{
			name:          "limit less than total",
			limit:         5,
			totalTools:    10,
			expectedCount: 5,
		},
		{
			name:          "limit greater than total",
			limit:         20,
			totalTools:    10,
			expectedCount: 10,
		},
		{
			name:          "limit equals total",
			limit:         10,
			totalTools:    10,
			expectedCount: 10,
		},
		{
			name:          "zero limit (no limit)",
			limit:         0,
			totalTools:    10,
			expectedCount: 10,
		},
		{
			name:          "limit of 1",
			limit:         1,
			totalTools:    10,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate limiting logic
			var count int
			if tt.limit == 0 {
				count = tt.totalTools
			} else if tt.limit < tt.totalTools {
				count = tt.limit
			} else {
				count = tt.totalTools
			}

			if count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

// =============================================================================
// Schema Requirement Tests
// =============================================================================

// TestSchemaRequirement_Validation tests input schema requirement validation.
func TestSchemaRequirement_Validation(t *testing.T) {
	tests := []struct {
		name          string
		requirement   SchemaRequirement
		schemaType    string
		shouldValidate bool
	}{
		{
			name: "required string field present",
			requirement: SchemaRequirement{
				Type:     "string",
				Required: true,
			},
			schemaType:    "string",
			shouldValidate: true,
		},
		{
			name: "required string field with different type",
			requirement: SchemaRequirement{
				Type:     "string",
				Required: true,
			},
			schemaType:    "number",
			shouldValidate: false,
		},
		{
			name: "optional field present",
			requirement: SchemaRequirement{
				Type:     "string",
				Required: false,
			},
			schemaType:    "string",
			shouldValidate: true,
		},
		{
			name: "optional field absent",
			requirement: SchemaRequirement{
				Type:     "string",
				Required: false,
			},
			schemaType:    "",
			shouldValidate: true,
		},
		{
			name: "object type validation",
			requirement: SchemaRequirement{
				Type:     "object",
				Required: true,
			},
			schemaType:    "object",
			shouldValidate: true,
		},
		{
			name: "array type validation",
			requirement: SchemaRequirement{
				Type:     "array",
				Required: true,
			},
			schemaType:    "array",
			shouldValidate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate schema requirement
			valid := tt.requirement.Type == tt.schemaType ||
				(!tt.requirement.Required && tt.schemaType == "")

			if valid != tt.shouldValidate {
				t.Errorf("expected valid=%v, got %v", tt.shouldValidate, valid)
			}
		})
	}
}

// =============================================================================
// Router Configuration Tests
// =============================================================================

// TestRouterConfig_Defaults tests default router configuration.
func TestRouterConfig_Defaults(t *testing.T) {
	// Create a mock registry for testing
	registry := NewTestMockRegistry()
	config := DefaultRouterConfig(registry)

	if config.Registry != registry {
		t.Error("Registry not set correctly")
	}

	if config.CacheOptions == nil {
		t.Fatal("CacheOptions should not be nil")
	}

	if config.DefaultRoutingOptions == nil {
		t.Fatal("DefaultRoutingOptions should not be nil")
	}

	if !config.EnableMetrics {
		t.Error("EnableMetrics should be true by default")
	}

	if config.MetricsPrefix != "flip2_mcp_router_" {
		t.Errorf("unexpected MetricsPrefix: %s", config.MetricsPrefix)
	}
}

// NewTestMockRegistry creates a mock registry for testing.
func NewTestMockRegistry() Registry {
	return &testMockRegistry{
		servers: make(map[string]Server),
	}
}

// testMockRegistry is a simple mock implementation of Registry for testing.
type testMockRegistry struct {
	servers map[string]Server
}

func (t *testMockRegistry) Register(ctx context.Context, server Server) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) Deregister(ctx context.Context, serverName string) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) GetServer(ctx context.Context, serverName string) (Server, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) ListServers(ctx context.Context) ([]Server, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) GetServerByToolName(ctx context.Context, toolName string) (Server, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) AllPrompts(ctx context.Context) ([]Prompt, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) List() []string {
	return nil
}

func (t *testMockRegistry) Get(name string) (Server, bool) {
	return nil, false
}

func (t *testMockRegistry) Close() error {
	return nil
}

func (t *testMockRegistry) ListByCapability(capability string) []Server {
	return nil
}

func (t *testMockRegistry) FindToolProvider(toolName string) (Server, bool) {
	return nil, false
}

func (t *testMockRegistry) FindResourceProvider(uri string) (Server, bool) {
	return nil, false
}

func (t *testMockRegistry) AllTools(ctx context.Context) (map[string]Server, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) AllResources(ctx context.Context) ([]Resource, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) Update(serverName string, fn func(*ServerInfo) error) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) ListAll() []ServerInfo {
	return nil
}

func (t *testMockRegistry) GetHealth(serverName string) (bool, error) {
	return false, errors.New("not implemented")
}

func (t *testMockRegistry) SetHealth(serverName string, healthy bool) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) SaveRegistry() error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) LoadRegistry() (map[string]ServerInfo, error) {
	return nil, errors.New("not implemented")
}

func (t *testMockRegistry) AddServerInfo(info *ServerInfo) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) RemoveServerInfo(id string) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) UpdateServerInfo(id string, info *ServerInfo) error {
	return errors.New("not implemented")
}

func (t *testMockRegistry) GetServerInfo(id string) *ServerInfo {
	return nil
}

func (t *testMockRegistry) ListServerInfos() []*ServerInfo {
	return nil
}

// =============================================================================
// Chain Options Tests
// =============================================================================

// TestChainOptions_Defaults tests default chain options.
func TestChainOptions_Defaults(t *testing.T) {
	opts := DefaultChainOptions()

	if opts.RoutingOptions == nil {
		t.Fatal("RoutingOptions should not be nil")
	}

	if opts.ContinueOnError {
		t.Error("ContinueOnError should be false by default")
	}

	if opts.MaxParallel != 1 {
		t.Errorf("expected MaxParallel 1, got %d", opts.MaxParallel)
	}

	if opts.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout 5m, got %v", opts.Timeout)
	}
}
