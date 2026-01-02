// Package mcp provides interfaces and types for Model Context Protocol (MCP) server integration.
//
// This file defines the ToolRouter interface for intelligent tool discovery and routing
// across multiple MCP servers. The router enables capability-based matching, ranking,
// and automatic tool selection for complex tasks.

package mcp

import (
	"context"
	"regexp"
	"time"
)

// =============================================================================
// Tool Router Interface
// =============================================================================

// ToolRouter provides intelligent tool discovery and routing across MCP servers.
//
// The router maintains an indexed view of all tools from registered MCP servers
// and provides sophisticated matching capabilities to find the best tool(s) for
// any given task. It supports:
//
//   - Capability-based discovery: Find tools that can perform specific actions
//   - Fuzzy matching: Match task descriptions to tool capabilities
//   - Multi-tool routing: Chain multiple tools for complex tasks
//   - Automatic fallback: Retry with alternative tools on failure
//   - Caching: Avoid repeated queries to servers for tool lists
//
// # Routing Algorithm
//
// The router uses a multi-stage matching algorithm:
//
//  1. Keyword Extraction: Parse the task description to identify intent
//     (verbs like "read", "write", "search") and objects ("file", "web", "database").
//
//  2. Capability Matching: Match keywords against tool names, descriptions,
//     and input schemas to find candidate tools.
//
//  3. Scoring: Rank candidates based on:
//     - Name similarity (exact match scores highest)
//     - Description relevance (semantic similarity)
//     - Schema compatibility (can the task inputs satisfy the tool?)
//     - Server reliability (historical success rate)
//     - Tool annotations (prefer read-only for queries, etc.)
//
//  4. Selection: Return ranked candidates or auto-select the best match.
//
// # Example Usage
//
//	router := mcp.NewToolRouter(registry)
//
//	// Find tools for a task
//	matches, err := router.FindTools(ctx, &mcp.ToolQuery{
//	    Description: "read the contents of a file",
//	    Capabilities: []string{"filesystem", "read"},
//	})
//
//	// Execute with automatic tool selection
//	result, err := router.Execute(ctx, &mcp.RoutedTask{
//	    Description: "search the web for golang tutorials",
//	    Arguments: map[string]any{"query": "golang tutorials"},
//	})
//
//	// Chain multiple tools for complex tasks
//	results, err := router.ExecuteChain(ctx, &mcp.ToolChain{
//	    Steps: []mcp.ChainStep{
//	        {Description: "read config file", Arguments: map[string]any{"path": "/etc/app.conf"}},
//	        {Description: "parse JSON content", DependsOn: 0},
//	    },
//	})
type ToolRouter interface {
	// =========================================================================
	// Tool Discovery
	// =========================================================================

	// FindTools searches for tools matching the given query.
	//
	// The query can include a natural language description, required capabilities,
	// or specific tool name patterns. Returns matches ranked by relevance.
	//
	// Example:
	//
	//	matches, err := router.FindTools(ctx, &ToolQuery{
	//	    Description: "read a file from disk",
	//	})
	//	// Returns: [ToolMatch{Tool: "read_file", Score: 0.95}, ...]
	FindTools(ctx context.Context, query *ToolQuery) ([]ToolMatch, error)

	// FindToolByName returns a specific tool by exact name.
	// Returns the tool and its server, or an error if not found.
	FindToolByName(ctx context.Context, name string) (*IndexedTool, error)

	// ListAllTools returns all tools from all registered servers.
	// Results are cached according to CacheOptions.
	ListAllTools(ctx context.Context) ([]IndexedTool, error)

	// ListToolsByServer returns all tools from a specific server.
	ListToolsByServer(ctx context.Context, serverName string) ([]IndexedTool, error)

	// ListToolsByCapability returns tools that match a capability tag.
	// Common capabilities: "filesystem", "web", "database", "api", "shell"
	ListToolsByCapability(ctx context.Context, capability string) ([]IndexedTool, error)

	// =========================================================================
	// Tool Execution
	// =========================================================================

	// Execute runs a task using the best matching tool.
	//
	// The router automatically selects the most appropriate tool based on the
	// task description and arguments. If the selected tool fails and fallback
	// is enabled, it will retry with alternative tools.
	//
	// Example:
	//
	//	result, err := router.Execute(ctx, &RoutedTask{
	//	    Description: "search for 'error' in log files",
	//	    Arguments: map[string]any{
	//	        "pattern": "error",
	//	        "path": "/var/log/*.log",
	//	    },
	//	    Options: &RoutingOptions{
	//	        EnableFallback: true,
	//	        MaxRetries: 3,
	//	    },
	//	})
	Execute(ctx context.Context, task *RoutedTask) (*RoutedResult, error)

	// ExecuteWithTool runs a task using a specific tool.
	// Bypasses automatic tool selection but still applies retry logic.
	ExecuteWithTool(ctx context.Context, toolName string, arguments map[string]any, options *RoutingOptions) (*RoutedResult, error)

	// ExecuteChain runs a sequence of tools, passing outputs between steps.
	//
	// Each step can reference outputs from previous steps. The chain stops
	// on the first error unless ContinueOnError is set.
	//
	// Example:
	//
	//	results, err := router.ExecuteChain(ctx, &ToolChain{
	//	    Steps: []ChainStep{
	//	        {Description: "read config", Arguments: map[string]any{"path": "config.json"}},
	//	        {Description: "validate JSON", InputTransform: "$.content"},
	//	    },
	//	})
	ExecuteChain(ctx context.Context, chain *ToolChain) (*ChainResult, error)

	// ExecuteParallel runs multiple independent tasks concurrently.
	// Returns results in the same order as the input tasks.
	ExecuteParallel(ctx context.Context, tasks []*RoutedTask) ([]*RoutedResult, error)

	// =========================================================================
	// Capability Analysis
	// =========================================================================

	// CanHandle checks if the router has tools capable of handling a task.
	// Returns a confidence score (0-1) and the best matching tool.
	CanHandle(ctx context.Context, description string) (*CapabilityCheck, error)

	// SuggestTools returns tool suggestions for a task with explanations.
	// Useful for interactive tool selection or debugging routing decisions.
	SuggestTools(ctx context.Context, description string, limit int) ([]ToolSuggestion, error)

	// AnalyzeTask breaks down a complex task into subtasks with tool assignments.
	// Used for planning multi-tool workflows.
	AnalyzeTask(ctx context.Context, description string) (*TaskAnalysis, error)

	// =========================================================================
	// Cache Management
	// =========================================================================

	// RefreshCache forces a refresh of the tool cache for all servers.
	RefreshCache(ctx context.Context) error

	// RefreshServerCache forces a refresh for a specific server.
	RefreshServerCache(ctx context.Context, serverName string) error

	// InvalidateCache clears the entire tool cache.
	InvalidateCache()

	// CacheStats returns statistics about cache usage.
	CacheStats() *CacheStatistics

	// =========================================================================
	// Event Handlers
	// =========================================================================

	// OnToolsChanged registers a callback for when any server's tools change.
	// The callback receives the server name that changed.
	OnToolsChanged(callback func(serverName string))

	// OnRoutingDecision registers a callback for observing routing decisions.
	// Useful for logging, debugging, or learning from routing patterns.
	OnRoutingDecision(callback func(decision *RoutingDecision))
}

// =============================================================================
// Query Types
// =============================================================================

// ToolQuery specifies criteria for finding tools.
type ToolQuery struct {
	// Description is a natural language description of the desired capability.
	// Example: "read file contents", "search the web", "execute shell command"
	Description string

	// Capabilities is a list of required capability tags.
	// Tools must match ALL specified capabilities.
	// Example: ["filesystem", "read"] matches tools that can read from filesystem
	Capabilities []string

	// NamePattern is a regex pattern to match tool names.
	// Optional; if set, only tools matching the pattern are considered.
	NamePattern *regexp.Regexp

	// ServerFilter limits the search to specific servers.
	// If empty, all servers are searched.
	ServerFilter []string

	// RequireAnnotations specifies required tool annotations.
	// Example: RequireReadOnly=true to only match non-destructive tools
	RequireAnnotations *AnnotationFilter

	// InputSchema specifies required input parameters.
	// Tools must accept ALL specified parameters.
	InputSchema map[string]SchemaRequirement

	// MinScore is the minimum relevance score (0-1) for results.
	// Default: 0.0 (return all matches)
	MinScore float64

	// Limit is the maximum number of results to return.
	// Default: 10
	Limit int
}

// AnnotationFilter specifies required tool annotations.
type AnnotationFilter struct {
	// RequireReadOnly only matches tools marked as read-only.
	RequireReadOnly bool

	// ExcludeDestructive excludes tools marked as destructive.
	ExcludeDestructive bool

	// RequireIdempotent only matches idempotent tools.
	RequireIdempotent bool

	// RequireOpenWorld only matches tools that interact with external systems.
	RequireOpenWorld bool
}

// SchemaRequirement specifies a required input parameter.
type SchemaRequirement struct {
	// Type is the required JSON Schema type ("string", "number", "boolean", "object", "array").
	Type string

	// Required indicates the parameter must be present in the schema.
	Required bool
}

// =============================================================================
// Match Types
// =============================================================================

// ToolMatch represents a tool that matches a query.
type ToolMatch struct {
	// Tool is the matched tool with server information.
	Tool IndexedTool

	// Score is the relevance score (0-1).
	// 1.0 = perfect match, 0.0 = no match
	Score float64

	// Breakdown provides detail on how the score was calculated.
	Breakdown *ScoreBreakdown

	// Reasoning explains why this tool was matched.
	Reasoning string
}

// IndexedTool combines a tool with its server context.
type IndexedTool struct {
	// Tool is the MCP tool definition.
	Tool Tool

	// ServerName is the name of the server providing this tool.
	ServerName string

	// Server is a reference to the server (for direct invocation).
	Server Server

	// Capabilities are inferred capability tags for this tool.
	// Derived from tool name, description, and annotations.
	Capabilities []string

	// LastUpdated is when this tool entry was last refreshed.
	LastUpdated time.Time
}

// ScoreBreakdown provides detail on scoring components.
type ScoreBreakdown struct {
	// NameScore is the score from name matching (0-1).
	NameScore float64

	// DescriptionScore is the score from description similarity (0-1).
	DescriptionScore float64

	// CapabilityScore is the score from capability matching (0-1).
	CapabilityScore float64

	// SchemaScore is the score from input schema compatibility (0-1).
	SchemaScore float64

	// AnnotationScore is the score from annotation matching (0-1).
	AnnotationScore float64

	// ReliabilityScore is the score from server reliability (0-1).
	ReliabilityScore float64

	// Weights are the weights applied to each component.
	Weights ScoreWeights
}

// ScoreWeights configures the importance of each scoring component.
type ScoreWeights struct {
	Name        float64 // Weight for name matching (default: 0.3)
	Description float64 // Weight for description similarity (default: 0.25)
	Capability  float64 // Weight for capability matching (default: 0.2)
	Schema      float64 // Weight for schema compatibility (default: 0.15)
	Annotation  float64 // Weight for annotation matching (default: 0.05)
	Reliability float64 // Weight for server reliability (default: 0.05)
}

// DefaultScoreWeights returns the default scoring weights.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		Name:        0.30,
		Description: 0.25,
		Capability:  0.20,
		Schema:      0.15,
		Annotation:  0.05,
		Reliability: 0.05,
	}
}

// =============================================================================
// Execution Types
// =============================================================================

// RoutedTask represents a task to be executed via the router.
type RoutedTask struct {
	// Description is a natural language description of the task.
	// Used for automatic tool selection.
	Description string

	// Arguments are the input parameters for the tool.
	Arguments map[string]any

	// PreferredTool is a hint for which tool to use.
	// If set and available, this tool is tried first.
	PreferredTool string

	// Options configures routing and execution behavior.
	Options *RoutingOptions
}

// RoutingOptions configures routing and execution behavior.
type RoutingOptions struct {
	// EnableFallback allows trying alternative tools on failure.
	// Default: true
	EnableFallback bool

	// MaxRetries is the maximum number of retry attempts per tool.
	// Default: 3
	MaxRetries int

	// RetryDelay is the initial delay between retries.
	// Subsequent retries use exponential backoff.
	// Default: 1 second
	RetryDelay time.Duration

	// MaxFallbackAttempts is the maximum number of alternative tools to try.
	// Default: 3
	MaxFallbackAttempts int

	// Timeout is the maximum time for the entire routing operation.
	// Default: 60 seconds
	Timeout time.Duration

	// ServerPreference prioritizes tools from specific servers.
	// Servers are tried in order of preference.
	ServerPreference []string

	// ScoreWeights overrides the default scoring weights.
	ScoreWeights *ScoreWeights

	// RequireAnnotations filters tools by annotations.
	RequireAnnotations *AnnotationFilter

	// DryRun returns the selected tool without executing.
	DryRun bool
}

// DefaultRoutingOptions returns sensible defaults.
func DefaultRoutingOptions() *RoutingOptions {
	return &RoutingOptions{
		EnableFallback:      true,
		MaxRetries:          3,
		RetryDelay:          time.Second,
		MaxFallbackAttempts: 3,
		Timeout:             60 * time.Second,
	}
}

// RoutedResult is the result of a routed task execution.
type RoutedResult struct {
	// Result is the tool execution result.
	Result *ToolResult

	// ToolUsed is the name of the tool that was executed.
	ToolUsed string

	// ServerUsed is the server that provided the tool.
	ServerUsed string

	// Attempts is the total number of attempts made.
	Attempts int

	// FallbacksUsed is the number of fallback tools tried.
	FallbacksUsed int

	// Duration is the total execution time.
	Duration time.Duration

	// RoutingDecision contains details about how the tool was selected.
	RoutingDecision *RoutingDecision
}

// RoutingDecision records the reasoning behind a routing choice.
type RoutingDecision struct {
	// TaskDescription is the original task description.
	TaskDescription string

	// Candidates are the tools that were considered.
	Candidates []ToolMatch

	// SelectedTool is the tool that was chosen.
	SelectedTool string

	// SelectedServer is the server providing the selected tool.
	SelectedServer string

	// SelectionReason explains why this tool was selected.
	SelectionReason string

	// RejectedTools lists tools that were considered but not selected.
	RejectedTools []RejectedTool

	// Timestamp is when the decision was made.
	Timestamp time.Time
}

// RejectedTool records why a tool was not selected.
type RejectedTool struct {
	// ToolName is the name of the rejected tool.
	ToolName string

	// ServerName is the server providing the tool.
	ServerName string

	// Reason explains why it was rejected.
	Reason string

	// Score is the tool's relevance score.
	Score float64
}

// =============================================================================
// Chain Types
// =============================================================================

// ToolChain represents a sequence of tools to execute.
type ToolChain struct {
	// Steps are the ordered steps in the chain.
	Steps []ChainStep

	// Options configures chain execution.
	Options *ChainOptions
}

// ChainStep represents a single step in a tool chain.
type ChainStep struct {
	// Description is a natural language description for automatic tool selection.
	Description string

	// ToolName is an explicit tool name (bypasses automatic selection).
	ToolName string

	// Arguments are the base input parameters.
	// Can reference previous step outputs using JSONPath syntax.
	Arguments map[string]any

	// InputTransform is a JSONPath expression to extract input from previous step.
	// Example: "$.content" extracts the content field from the previous result.
	InputTransform string

	// DependsOn specifies which previous steps this step depends on.
	// Step indices are 0-based. If empty, depends on the immediately previous step.
	DependsOn []int

	// StepOptions overrides chain-level options for this step.
	StepOptions *RoutingOptions
}

// ChainOptions configures chain execution behavior.
type ChainOptions struct {
	// RoutingOptions are the default options for each step.
	RoutingOptions *RoutingOptions

	// ContinueOnError continues execution even if a step fails.
	// The failed step's result will have IsError=true.
	// Default: false
	ContinueOnError bool

	// MaxParallel is the maximum number of steps to run in parallel.
	// Steps with no dependencies or satisfied dependencies can run in parallel.
	// Default: 1 (sequential)
	MaxParallel int

	// Timeout is the maximum time for the entire chain.
	// Default: 5 minutes
	Timeout time.Duration
}

// DefaultChainOptions returns sensible defaults.
func DefaultChainOptions() *ChainOptions {
	return &ChainOptions{
		RoutingOptions:  DefaultRoutingOptions(),
		ContinueOnError: false,
		MaxParallel:     1,
		Timeout:         5 * time.Minute,
	}
}

// ChainResult is the result of executing a tool chain.
type ChainResult struct {
	// StepResults are the results from each step, in order.
	StepResults []*StepResult

	// Success is true if all steps completed successfully.
	Success bool

	// FailedStep is the index of the first failed step (-1 if all succeeded).
	FailedStep int

	// Duration is the total execution time.
	Duration time.Duration
}

// StepResult is the result of a single chain step.
type StepResult struct {
	// StepIndex is the 0-based index of this step.
	StepIndex int

	// RoutedResult is the execution result.
	RoutedResult *RoutedResult

	// Error is set if the step failed.
	Error error

	// Duration is the step execution time.
	Duration time.Duration
}

// =============================================================================
// Capability Types
// =============================================================================

// CapabilityCheck is the result of checking if a task can be handled.
type CapabilityCheck struct {
	// CanHandle is true if there's at least one capable tool.
	CanHandle bool

	// Confidence is the confidence score (0-1).
	Confidence float64

	// BestMatch is the highest-scoring tool match.
	BestMatch *ToolMatch

	// AlternativeCount is the number of alternative tools available.
	AlternativeCount int

	// MissingCapabilities lists capabilities needed but not available.
	MissingCapabilities []string
}

// ToolSuggestion is a tool recommendation with explanation.
type ToolSuggestion struct {
	// Tool is the suggested tool.
	Tool IndexedTool

	// Score is the relevance score.
	Score float64

	// Explanation is a human-readable explanation of why this tool is suggested.
	Explanation string

	// UsageExample shows how to use this tool for the task.
	UsageExample string

	// Caveats lists potential issues or limitations.
	Caveats []string
}

// TaskAnalysis breaks down a complex task into subtasks.
type TaskAnalysis struct {
	// OriginalTask is the input task description.
	OriginalTask string

	// Subtasks are the identified subtasks.
	Subtasks []Subtask

	// SuggestedChain is a recommended tool chain for the task.
	SuggestedChain *ToolChain

	// Feasibility indicates if the task can be completed with available tools.
	Feasibility TaskFeasibility

	// MissingCapabilities lists capabilities needed but not available.
	MissingCapabilities []string
}

// Subtask represents a component of a complex task.
type Subtask struct {
	// Description describes this subtask.
	Description string

	// Order is the execution order (1-based).
	Order int

	// Dependencies are indices of subtasks this depends on.
	Dependencies []int

	// RecommendedTools are tools that could handle this subtask.
	RecommendedTools []ToolMatch

	// Required indicates if this subtask is essential.
	Required bool
}

// TaskFeasibility indicates how feasible a task is.
type TaskFeasibility string

const (
	// FeasibilityFull means the task can be fully completed.
	FeasibilityFull TaskFeasibility = "full"

	// FeasibilityPartial means some subtasks can be completed.
	FeasibilityPartial TaskFeasibility = "partial"

	// FeasibilityNone means no tools are available for the task.
	FeasibilityNone TaskFeasibility = "none"
)

// =============================================================================
// Cache Types
// =============================================================================

// CacheStatistics provides information about cache usage.
type CacheStatistics struct {
	// TotalTools is the total number of cached tools.
	TotalTools int

	// ServerCount is the number of servers with cached tools.
	ServerCount int

	// HitCount is the number of cache hits since last reset.
	HitCount int64

	// MissCount is the number of cache misses since last reset.
	MissCount int64

	// HitRate is the cache hit rate (0-1).
	HitRate float64

	// LastRefresh is when the cache was last fully refreshed.
	LastRefresh time.Time

	// OldestEntry is the age of the oldest cache entry.
	OldestEntry time.Duration

	// MemoryUsage is the estimated memory usage in bytes.
	MemoryUsage int64
}

// CacheOptions configures the tool cache behavior.
type CacheOptions struct {
	// TTL is the time-to-live for cache entries.
	// Default: 5 minutes
	TTL time.Duration

	// MaxSize is the maximum number of tools to cache.
	// Default: 10000
	MaxSize int

	// RefreshOnChange enables automatic refresh when servers report changes.
	// Default: true
	RefreshOnChange bool

	// PrefetchOnInit loads all tools into cache on router initialization.
	// Default: true
	PrefetchOnInit bool
}

// DefaultCacheOptions returns sensible defaults.
func DefaultCacheOptions() *CacheOptions {
	return &CacheOptions{
		TTL:             5 * time.Minute,
		MaxSize:         10000,
		RefreshOnChange: true,
		PrefetchOnInit:  true,
	}
}

// =============================================================================
// Router Configuration
// =============================================================================

// RouterConfig configures the ToolRouter implementation.
type RouterConfig struct {
	// Registry is the MCP server registry to route across.
	Registry Registry

	// CacheOptions configures tool caching.
	CacheOptions *CacheOptions

	// DefaultRoutingOptions are the default options for task execution.
	DefaultRoutingOptions *RoutingOptions

	// ScoreWeights configures the default scoring weights.
	ScoreWeights *ScoreWeights

	// CapabilityExtractor is a function that extracts capabilities from tool metadata.
	// If nil, a default extractor based on name/description parsing is used.
	CapabilityExtractor func(tool Tool) []string

	// CustomScorer is a function that provides custom scoring logic.
	// If nil, the default scoring algorithm is used.
	// The function receives the query and tool, returns a score (0-1).
	CustomScorer func(query *ToolQuery, tool IndexedTool) float64

	// EnableMetrics enables collection of routing metrics.
	EnableMetrics bool

	// MetricsPrefix is the prefix for metric names.
	MetricsPrefix string
}

// DefaultRouterConfig returns sensible defaults.
func DefaultRouterConfig(registry Registry) *RouterConfig {
	return &RouterConfig{
		Registry:              registry,
		CacheOptions:          DefaultCacheOptions(),
		DefaultRoutingOptions: DefaultRoutingOptions(),
		ScoreWeights:          nil, // Use DefaultScoreWeights()
		EnableMetrics:         true,
		MetricsPrefix:         "flip2_mcp_router_",
	}
}

// =============================================================================
// Well-Known Capabilities
// =============================================================================

// Common capability tags used for tool classification.
// These are inferred from tool names and descriptions.
const (
	// CapabilityFilesystem indicates file/directory operations.
	CapabilityFilesystem = "filesystem"

	// CapabilityRead indicates read-only data access.
	CapabilityRead = "read"

	// CapabilityWrite indicates data modification.
	CapabilityWrite = "write"

	// CapabilityDelete indicates data deletion.
	CapabilityDelete = "delete"

	// CapabilitySearch indicates search/query functionality.
	CapabilitySearch = "search"

	// CapabilityWeb indicates web/HTTP operations.
	CapabilityWeb = "web"

	// CapabilityBrowser indicates browser automation.
	CapabilityBrowser = "browser"

	// CapabilityAPI indicates external API interaction.
	CapabilityAPI = "api"

	// CapabilityDatabase indicates database operations.
	CapabilityDatabase = "database"

	// CapabilityShell indicates shell/command execution.
	CapabilityShell = "shell"

	// CapabilityGit indicates git operations.
	CapabilityGit = "git"

	// CapabilityAI indicates AI/LLM operations.
	CapabilityAI = "ai"

	// CapabilityImage indicates image processing.
	CapabilityImage = "image"

	// CapabilityAudio indicates audio processing.
	CapabilityAudio = "audio"

	// CapabilityEmail indicates email operations.
	CapabilityEmail = "email"

	// CapabilityCalendar indicates calendar operations.
	CapabilityCalendar = "calendar"

	// CapabilityMessaging indicates messaging/chat operations.
	CapabilityMessaging = "messaging"
)

// =============================================================================
// Error Types
// =============================================================================

// RouterError represents a routing-specific error.
type RouterError struct {
	// Code is the error code.
	Code RouterErrorCode

	// Message is the error description.
	Message string

	// ToolName is the tool that caused the error (if applicable).
	ToolName string

	// ServerName is the server that caused the error (if applicable).
	ServerName string

	// Cause is the underlying error.
	Cause error

	// Attempts is the number of attempts made before failing.
	Attempts int

	// AttemptedTools lists all tools that were tried.
	AttemptedTools []string
}

// Error implements the error interface.
func (e *RouterError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *RouterError) Unwrap() error {
	return e.Cause
}

// RouterErrorCode represents router error types.
type RouterErrorCode string

const (
	// ErrNoMatchingTool indicates no tool matches the query.
	ErrNoMatchingTool RouterErrorCode = "no_matching_tool"

	// ErrAllToolsFailed indicates all attempted tools failed.
	ErrAllToolsFailed RouterErrorCode = "all_tools_failed"

	// ErrToolNotFound indicates a specific tool was not found.
	ErrToolNotFound RouterErrorCode = "tool_not_found"

	// ErrServerUnavailable indicates a server is not responding.
	ErrServerUnavailable RouterErrorCode = "server_unavailable"

	// ErrInvalidArguments indicates arguments don't match the tool schema.
	ErrInvalidArguments RouterErrorCode = "invalid_arguments"

	// ErrTimeout indicates the operation timed out.
	ErrTimeout RouterErrorCode = "timeout"

	// ErrChainFailed indicates a step in a tool chain failed.
	ErrChainFailed RouterErrorCode = "chain_failed"

	// ErrCacheMiss indicates a cache lookup failed unexpectedly.
	ErrCacheMiss RouterErrorCode = "cache_miss"
)
