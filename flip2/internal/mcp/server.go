// Package mcp provides interfaces and types for Model Context Protocol (MCP) server integration.
//
// The Model Context Protocol is a standardized way for applications to provide context
// to Large Language Models (LLMs). This package defines the Go interfaces that abstract
// MCP server connections, enabling FLIP2 to connect to the 77k+ existing MCP servers
// in the ecosystem.
//
// # Architecture Overview
//
// FLIP2 acts as an MCP client that can connect to multiple MCP servers simultaneously.
// Each server can expose:
//   - Tools: Callable functions with defined input/output schemas
//   - Resources: Readable data sources (files, databases, APIs)
//   - Prompts: Reusable prompt templates with arguments
//
// Additionally, servers can request LLM completions from FLIP2 via the Sampling interface,
// allowing servers to leverage AI capabilities without needing their own API keys.
//
// # Protocol Versions
//
// This implementation supports MCP protocol versions:
//   - 2024-11-05 (baseline)
//   - 2025-03-26 (adds elicitation)
//   - 2025-06-18 (latest, adds audio/image content)
//
// # Example Usage
//
//	// Create a server connection
//	server, err := mcp.NewStdioServer("npx", "-y", "@modelcontextprotocol/server-filesystem", "/path")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Close()
//
//	// Initialize the connection
//	caps, err := server.Initialize(ctx, &mcp.ClientInfo{
//	    Name:    "flip2",
//	    Version: "2.0.0",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List available tools
//	tools, err := server.ListTools(ctx, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Call a tool
//	result, err := server.CallTool(ctx, "read_file", map[string]any{"path": "/etc/hosts"})
//	if err != nil {
//	    log.Fatal(err)
//	}
package mcp

import (
	"context"
	"encoding/json"
	"time"
)

// ProtocolVersion represents a supported MCP protocol version.
type ProtocolVersion string

const (
	// ProtocolVersion20241105 is the baseline MCP protocol version.
	ProtocolVersion20241105 ProtocolVersion = "2024-11-05"

	// ProtocolVersion20250326 adds elicitation support.
	ProtocolVersion20250326 ProtocolVersion = "2025-03-26"

	// ProtocolVersion20250618 is the latest version with audio/image content.
	ProtocolVersion20250618 ProtocolVersion = "2025-06-18"

	// LatestProtocolVersion is the most recent supported version.
	LatestProtocolVersion = ProtocolVersion20250618
)

// =============================================================================
// Core Server Interface
// =============================================================================

// Server represents a connection to an MCP server.
//
// Server is the primary interface for interacting with MCP servers. It provides
// methods for the complete MCP lifecycle: initialization, capability discovery,
// tool invocation, resource access, and prompt retrieval.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Server interface {
	// Lifecycle methods

	// Initialize performs the MCP handshake with the server.
	// This must be called before any other method.
	// Returns the server's capabilities and information.
	Initialize(ctx context.Context, clientInfo *ClientInfo) (*InitializeResult, error)

	// Ping checks if the server is responsive.
	// Returns nil if the server responds, error otherwise.
	Ping(ctx context.Context) error

	// Close terminates the connection to the server.
	// After Close is called, the server instance must not be reused.
	Close() error

	// Capability discovery

	// Capabilities returns the server's declared capabilities.
	// Must be called after Initialize.
	Capabilities() *ServerCapabilities

	// ServerInfo returns information about the connected server.
	// Must be called after Initialize.
	ServerInfo() *ServerInfo

	// Tool operations

	// ListTools returns all tools exposed by the server.
	// Pass nil cursor for the first page; use returned cursor for pagination.
	ListTools(ctx context.Context, cursor *string) (*ListToolsResult, error)

	// CallTool invokes a tool with the given arguments.
	// Arguments must conform to the tool's inputSchema.
	CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error)

	// Resource operations

	// ListResources returns all resources exposed by the server.
	// Pass nil cursor for the first page; use returned cursor for pagination.
	ListResources(ctx context.Context, cursor *string) (*ListResourcesResult, error)

	// ListResourceTemplates returns URI templates for dynamic resources.
	ListResourceTemplates(ctx context.Context, cursor *string) (*ListResourceTemplatesResult, error)

	// ReadResource retrieves the contents of a resource by URI.
	ReadResource(ctx context.Context, uri string) (*ResourceContents, error)

	// SubscribeResource subscribes to changes for a resource.
	// Returns a channel that receives updates when the resource changes.
	// The channel is closed when the subscription ends or on error.
	SubscribeResource(ctx context.Context, uri string) (<-chan *ResourceUpdate, error)

	// UnsubscribeResource cancels a resource subscription.
	UnsubscribeResource(ctx context.Context, uri string) error

	// Prompt operations

	// ListPrompts returns all prompt templates exposed by the server.
	// Pass nil cursor for the first page; use returned cursor for pagination.
	ListPrompts(ctx context.Context, cursor *string) (*ListPromptsResult, error)

	// GetPrompt retrieves a prompt template with the given arguments.
	GetPrompt(ctx context.Context, name string, arguments map[string]string) (*PromptResult, error)

	// Completion operations

	// CompleteArgument requests argument autocompletion from the server.
	// Used for interactive argument completion in CLIs or UIs.
	CompleteArgument(ctx context.Context, ref CompletionRef, argument CompletionArgument) (*CompletionResult, error)

	// Notification handling

	// OnToolsChanged registers a callback for tool list change notifications.
	// The callback is invoked when the server's tool list changes.
	OnToolsChanged(callback func())

	// OnResourcesChanged registers a callback for resource list change notifications.
	OnResourcesChanged(callback func())

	// OnPromptsChanged registers a callback for prompt list change notifications.
	OnPromptsChanged(callback func())

	// OnLog registers a callback for log messages from the server.
	OnLog(callback func(LogMessage))
}

// =============================================================================
// Sampling Interface (Server -> Client LLM Requests)
// =============================================================================

// SamplingHandler handles LLM completion requests from MCP servers.
//
// When an MCP server needs to use an LLM, it sends a sampling request to the
// client (FLIP2). The SamplingHandler processes these requests using FLIP2's
// configured LLM backends.
//
// This allows MCP servers to leverage AI capabilities without requiring their
// own API keys, while FLIP2 maintains control over model selection and costs.
type SamplingHandler interface {
	// CreateMessage handles an LLM completion request from a server.
	// The handler should use FLIP2's LLM backends to generate the response.
	CreateMessage(ctx context.Context, request *SamplingRequest) (*SamplingResponse, error)
}

// SamplingRequest represents an LLM completion request from an MCP server.
type SamplingRequest struct {
	// Messages is the conversation history to complete.
	Messages []SamplingMessage `json:"messages"`

	// ModelPreferences guides model selection.
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`

	// SystemPrompt provides instructions for the model.
	SystemPrompt string `json:"systemPrompt,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `json:"maxTokens,omitempty"`

	// StopSequences causes generation to stop when encountered.
	StopSequences []string `json:"stopSequences,omitempty"`

	// Metadata contains additional context.
	Metadata map[string]any `json:"metadata,omitempty"`

	// IncludeContext specifies what MCP context to include.
	// Values: "none", "thisServer", "allServers"
	IncludeContext string `json:"includeContext,omitempty"`
}

// SamplingMessage represents a message in a sampling conversation.
type SamplingMessage struct {
	// Role is "user" or "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content MessageContent `json:"content"`
}

// MessageContent represents content in a message.
// It can be text, image, or audio.
type MessageContent struct {
	// Type is "text", "image", or "audio".
	Type string `json:"type"`

	// Text is set when Type is "text".
	Text string `json:"text,omitempty"`

	// Data is base64-encoded data for image/audio.
	Data string `json:"data,omitempty"`

	// MimeType is set for image/audio content.
	MimeType string `json:"mimeType,omitempty"`
}

// ModelPreferences guides model selection for sampling requests.
type ModelPreferences struct {
	// Hints suggests specific models by name.
	Hints []ModelHint `json:"hints,omitempty"`

	// IntelligencePriority (0-1) prioritizes model capability.
	IntelligencePriority float64 `json:"intelligencePriority,omitempty"`

	// SpeedPriority (0-1) prioritizes response speed.
	SpeedPriority float64 `json:"speedPriority,omitempty"`

	// CostPriority (0-1) prioritizes lower cost.
	CostPriority float64 `json:"costPriority,omitempty"`
}

// ModelHint suggests a specific model.
type ModelHint struct {
	// Name is the model name (e.g., "claude-3-sonnet").
	Name string `json:"name"`
}

// SamplingResponse is the result of a sampling request.
type SamplingResponse struct {
	// Role is "assistant".
	Role string `json:"role"`

	// Content is the generated content.
	Content MessageContent `json:"content"`

	// Model is the model that generated the response.
	Model string `json:"model"`

	// StopReason indicates why generation stopped.
	// Values: "endTurn", "stopSequence", "maxTokens"
	StopReason string `json:"stopReason"`
}

// =============================================================================
// Initialization Types
// =============================================================================

// ClientInfo identifies the MCP client (FLIP2) to servers.
type ClientInfo struct {
	// Name is the client identifier (e.g., "flip2").
	Name string `json:"name"`

	// Version is the client version (e.g., "2.0.0").
	Version string `json:"version"`

	// Title is an optional display name.
	Title string `json:"title,omitempty"`
}

// ClientCapabilities declares what the client supports.
type ClientCapabilities struct {
	// Roots indicates filesystem root support.
	Roots *RootsCapability `json:"roots,omitempty"`

	// Sampling indicates LLM sampling support.
	Sampling *SamplingCapability `json:"sampling,omitempty"`

	// Elicitation indicates user elicitation support.
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`

	// Experimental contains non-standard capabilities.
	Experimental map[string]any `json:"experimental,omitempty"`
}

// RootsCapability declares filesystem root capabilities.
type RootsCapability struct {
	// ListChanged indicates support for root list change notifications.
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability declares sampling capabilities.
type SamplingCapability struct{}

// ElicitationCapability declares elicitation capabilities.
type ElicitationCapability struct{}

// ServerInfo contains information about an MCP server.
type ServerInfo struct {
	// Name is the server identifier.
	Name string `json:"name"`

	// Version is the server version.
	Version string `json:"version"`

	// Title is an optional display name.
	Title string `json:"title,omitempty"`
}

// ServerCapabilities declares what the server supports.
type ServerCapabilities struct {
	// Prompts indicates prompt template support.
	Prompts *PromptsCapability `json:"prompts,omitempty"`

	// Resources indicates resource support.
	Resources *ResourcesCapability `json:"resources,omitempty"`

	// Tools indicates tool support.
	Tools *ToolsCapability `json:"tools,omitempty"`

	// Logging indicates log message support.
	Logging *LoggingCapability `json:"logging,omitempty"`

	// Completions indicates argument completion support.
	Completions *CompletionsCapability `json:"completions,omitempty"`

	// Experimental contains non-standard capabilities.
	Experimental map[string]any `json:"experimental,omitempty"`
}

// PromptsCapability declares prompt capabilities.
type PromptsCapability struct {
	// ListChanged indicates support for prompt list change notifications.
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability declares resource capabilities.
type ResourcesCapability struct {
	// Subscribe indicates support for resource subscriptions.
	Subscribe bool `json:"subscribe,omitempty"`

	// ListChanged indicates support for resource list change notifications.
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability declares tool capabilities.
type ToolsCapability struct {
	// ListChanged indicates support for tool list change notifications.
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability declares logging capabilities.
type LoggingCapability struct{}

// CompletionsCapability declares completion capabilities.
type CompletionsCapability struct{}

// InitializeResult is the response from server initialization.
type InitializeResult struct {
	// ProtocolVersion is the negotiated protocol version.
	ProtocolVersion ProtocolVersion `json:"protocolVersion"`

	// Capabilities are the server's capabilities.
	Capabilities *ServerCapabilities `json:"capabilities"`

	// ServerInfo contains server identification.
	ServerInfo *ServerInfo `json:"serverInfo"`

	// Instructions are optional guidance for the client.
	Instructions string `json:"instructions,omitempty"`
}

// =============================================================================
// Tool Types
// =============================================================================

// Tool represents a callable function exposed by an MCP server.
type Tool struct {
	// Name is the unique identifier for the tool.
	Name string `json:"name"`

	// Description explains what the tool does.
	Description string `json:"description,omitempty"`

	// InputSchema is the JSON Schema for the tool's arguments.
	InputSchema json.RawMessage `json:"inputSchema"`

	// Annotations provides additional metadata.
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations provides metadata about a tool's behavior.
type ToolAnnotations struct {
	// Title is a human-readable name.
	Title string `json:"title,omitempty"`

	// ReadOnlyHint indicates the tool doesn't modify state.
	ReadOnlyHint bool `json:"readOnlyHint,omitempty"`

	// DestructiveHint indicates the tool may cause data loss.
	DestructiveHint bool `json:"destructiveHint,omitempty"`

	// IdempotentHint indicates repeated calls have the same effect.
	IdempotentHint bool `json:"idempotentHint,omitempty"`

	// OpenWorldHint indicates the tool interacts with external systems.
	OpenWorldHint bool `json:"openWorldHint,omitempty"`
}

// ListToolsResult is the response from listing tools.
type ListToolsResult struct {
	// Tools is the list of available tools.
	Tools []Tool `json:"tools"`

	// NextCursor is set if there are more results.
	NextCursor *string `json:"nextCursor,omitempty"`
}

// ToolResult is the response from calling a tool.
type ToolResult struct {
	// Content is the tool's output.
	Content []ContentItem `json:"content"`

	// IsError indicates if the tool execution failed.
	IsError bool `json:"isError,omitempty"`
}

// ContentItem represents a piece of content in a result.
type ContentItem struct {
	// Type is "text", "image", "audio", or "resource".
	Type string `json:"type"`

	// Text is set when Type is "text".
	Text string `json:"text,omitempty"`

	// Data is base64-encoded data for image/audio.
	Data string `json:"data,omitempty"`

	// MimeType is set for image/audio content.
	MimeType string `json:"mimeType,omitempty"`

	// Resource is set when Type is "resource".
	Resource *EmbeddedResource `json:"resource,omitempty"`
}

// EmbeddedResource represents a resource embedded in a result.
type EmbeddedResource struct {
	// URI identifies the resource.
	URI string `json:"uri"`

	// MimeType is the content type.
	MimeType string `json:"mimeType,omitempty"`

	// Text is the resource content (for text resources).
	Text string `json:"text,omitempty"`

	// Blob is base64-encoded binary content.
	Blob string `json:"blob,omitempty"`
}

// =============================================================================
// Resource Types
// =============================================================================

// Resource represents a readable data source exposed by an MCP server.
type Resource struct {
	// URI uniquely identifies the resource.
	URI string `json:"uri"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Description explains the resource.
	Description string `json:"description,omitempty"`

	// MimeType is the content type.
	MimeType string `json:"mimeType,omitempty"`

	// Size is the content size in bytes (if known).
	Size *int64 `json:"size,omitempty"`

	// Annotations provides additional metadata.
	Annotations *ResourceAnnotations `json:"annotations,omitempty"`
}

// ResourceAnnotations provides metadata about a resource.
type ResourceAnnotations struct {
	// Audience suggests who the resource is intended for.
	// Values: "user", "assistant"
	Audience []string `json:"audience,omitempty"`

	// Priority suggests importance (0-1).
	Priority float64 `json:"priority,omitempty"`
}

// ResourceTemplate is a URI template for dynamic resources.
type ResourceTemplate struct {
	// URITemplate is an RFC 6570 URI template.
	URITemplate string `json:"uriTemplate"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Description explains the template.
	Description string `json:"description,omitempty"`

	// MimeType is the content type of resources from this template.
	MimeType string `json:"mimeType,omitempty"`

	// Annotations provides additional metadata.
	Annotations *ResourceAnnotations `json:"annotations,omitempty"`
}

// ListResourcesResult is the response from listing resources.
type ListResourcesResult struct {
	// Resources is the list of available resources.
	Resources []Resource `json:"resources"`

	// NextCursor is set if there are more results.
	NextCursor *string `json:"nextCursor,omitempty"`
}

// ListResourceTemplatesResult is the response from listing resource templates.
type ListResourceTemplatesResult struct {
	// ResourceTemplates is the list of available templates.
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`

	// NextCursor is set if there are more results.
	NextCursor *string `json:"nextCursor,omitempty"`
}

// ResourceContents is the response from reading a resource.
type ResourceContents struct {
	// Contents is the list of content items.
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents the content of a resource.
type ResourceContent struct {
	// URI identifies the resource.
	URI string `json:"uri"`

	// MimeType is the content type.
	MimeType string `json:"mimeType,omitempty"`

	// Text is the content (for text resources).
	Text string `json:"text,omitempty"`

	// Blob is base64-encoded binary content.
	Blob string `json:"blob,omitempty"`
}

// ResourceUpdate is sent when a subscribed resource changes.
type ResourceUpdate struct {
	// URI identifies the changed resource.
	URI string `json:"uri"`

	// Contents is the new content (if available).
	Contents *ResourceContents `json:"contents,omitempty"`

	// Deleted indicates the resource was deleted.
	Deleted bool `json:"deleted,omitempty"`

	// Error is set if the update failed.
	Error error `json:"error,omitempty"`

	// Timestamp is when the update occurred.
	Timestamp time.Time `json:"timestamp"`
}

// =============================================================================
// Prompt Types
// =============================================================================

// Prompt represents a reusable prompt template.
type Prompt struct {
	// Name uniquely identifies the prompt.
	Name string `json:"name"`

	// Description explains the prompt.
	Description string `json:"description,omitempty"`

	// Arguments defines the template parameters.
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument defines a parameter for a prompt template.
type PromptArgument struct {
	// Name is the argument identifier.
	Name string `json:"name"`

	// Description explains the argument.
	Description string `json:"description,omitempty"`

	// Required indicates if the argument must be provided.
	Required bool `json:"required,omitempty"`
}

// ListPromptsResult is the response from listing prompts.
type ListPromptsResult struct {
	// Prompts is the list of available prompts.
	Prompts []Prompt `json:"prompts"`

	// NextCursor is set if there are more results.
	NextCursor *string `json:"nextCursor,omitempty"`
}

// PromptResult is the response from getting a prompt.
type PromptResult struct {
	// Description explains the prompt.
	Description string `json:"description,omitempty"`

	// Messages is the prompt content.
	Messages []PromptMessage `json:"messages"`
}

// PromptMessage is a message in a prompt result.
type PromptMessage struct {
	// Role is "user" or "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content MessageContent `json:"content"`
}

// =============================================================================
// Completion Types
// =============================================================================

// CompletionRef identifies what to complete.
type CompletionRef struct {
	// Type is "ref/prompt" or "ref/resource".
	Type string `json:"type"`

	// Name is the prompt or resource template name.
	Name string `json:"name,omitempty"`

	// URI is the resource URI.
	URI string `json:"uri,omitempty"`
}

// CompletionArgument identifies the argument to complete.
type CompletionArgument struct {
	// Name is the argument name.
	Name string `json:"name"`

	// Value is the current partial value.
	Value string `json:"value"`
}

// CompletionResult is the response from argument completion.
type CompletionResult struct {
	// Completion contains the completion options.
	Completion CompletionOptions `json:"completion"`
}

// CompletionOptions contains completion suggestions.
type CompletionOptions struct {
	// Values is the list of completion options.
	Values []string `json:"values"`

	// Total is the total number of available completions.
	Total *int `json:"total,omitempty"`

	// HasMore indicates if there are more completions.
	HasMore bool `json:"hasMore,omitempty"`
}

// =============================================================================
// Logging Types
// =============================================================================

// LogLevel represents the severity of a log message.
type LogLevel string

const (
	LogLevelDebug     LogLevel = "debug"
	LogLevelInfo      LogLevel = "info"
	LogLevelNotice    LogLevel = "notice"
	LogLevelWarning   LogLevel = "warning"
	LogLevelError     LogLevel = "error"
	LogLevelCritical  LogLevel = "critical"
	LogLevelAlert     LogLevel = "alert"
	LogLevelEmergency LogLevel = "emergency"
)

// LogMessage represents a log entry from an MCP server.
type LogMessage struct {
	// Level is the log severity.
	Level LogLevel `json:"level"`

	// Logger identifies the log source.
	Logger string `json:"logger,omitempty"`

	// Data is the log content.
	Data any `json:"data"`
}

// =============================================================================
// Transport Interface
// =============================================================================

// Transport abstracts the communication layer with an MCP server.
//
// Implementations handle the JSON-RPC 2.0 message framing and delivery
// over different transport mechanisms (stdio, HTTP, WebSocket, etc.).
type Transport interface {
	// Send sends a JSON-RPC request and waits for the response.
	Send(ctx context.Context, method string, params any) (json.RawMessage, error)

	// Notify sends a JSON-RPC notification (no response expected).
	Notify(ctx context.Context, method string, params any) error

	// OnNotification registers a handler for incoming notifications.
	OnNotification(handler func(method string, params json.RawMessage))

	// Close terminates the transport connection.
	Close() error
}

// =============================================================================
// Registry Interface
// =============================================================================

// Registry manages a collection of MCP server connections.
//
// The registry provides centralized management of server connections,
// including registration, discovery, and lifecycle management.
type Registry interface {
	// Register adds a server to the registry.
	// The server should already be initialized.
	Register(ctx context.Context, server Server) error

	// Deregister removes a server from the registry and closes its connection.
	Deregister(ctx context.Context, serverName string) error

	// Get retrieves a server by name.
	Get(serverName string) (Server, bool)

	// List returns all registered server names.
	List() []string

	// ListByCapability returns servers that have a specific capability.
	// Capability names: "tools", "resources", "prompts", "sampling", "logging"
	ListByCapability(capability string) []Server

	// FindToolProvider finds a server that provides a specific tool.
	FindToolProvider(toolName string) (Server, bool)

	// FindResourceProvider finds a server that can provide a resource.
	FindResourceProvider(uri string) (Server, bool)

	// AllTools returns all tools from all registered servers.
	// Returns a map of tool name to the server that provides it.
	AllTools(ctx context.Context) (map[string]Server, error)

	// AllResources returns all resources from all registered servers.
	AllResources(ctx context.Context) ([]Resource, error)

	// AllPrompts returns all prompts from all registered servers.
	AllPrompts(ctx context.Context) ([]Prompt, error)

	// Update applies a custom update function to a server's metadata.
	// The updateFn receives the current ServerInfo and can modify its fields.
	// Returns an error if the server is not found or the updateFn fails.
	Update(serverName string, updateFn func(*ServerInfo) error) error

	// ListAll returns detailed information about all registered servers.
	// Returns a slice of ServerInfo structs with their health status.
	ListAll() []ServerInfo

	// GetHealth checks the health status of a server.
	// Returns true if the server is healthy (last Ping succeeded), false if unhealthy.
	// Returns an error if the server is not found.
	GetHealth(serverName string) (bool, error)

	// SetHealth updates the health status of a server.
	// Returns an error if the server is not found.
	SetHealth(serverName string, healthy bool) error

	// SaveRegistry persists the current registry state to SQLite.
	// Only saves server metadata and health status, not the active Server instances.
	// Returns an error if persistence is not enabled or if the database operation fails.
	SaveRegistry() error

	// LoadRegistry loads server metadata and health status from SQLite.
	// Does not restore Server instances - those must be reconstructed externally.
	// Returns a map of server names to their metadata and restores health status.
	// Returns an error if persistence is not enabled or if the database operation fails.
	LoadRegistry() (map[string]ServerInfo, error)

	// Close closes all server connections.
	Close() error
}

// =============================================================================
// Error Types
// =============================================================================

// ErrorCode represents MCP JSON-RPC error codes.
type ErrorCode int

const (
	// Standard JSON-RPC error codes
	ErrorCodeParseError     ErrorCode = -32700
	ErrorCodeInvalidRequest ErrorCode = -32600
	ErrorCodeMethodNotFound ErrorCode = -32601
	ErrorCodeInvalidParams  ErrorCode = -32602
	ErrorCodeInternalError  ErrorCode = -32603

	// MCP-specific error codes
	ErrorCodeResourceNotFound ErrorCode = -32002
	ErrorCodeToolNotFound     ErrorCode = -32003
	ErrorCodePromptNotFound   ErrorCode = -32004
)

// Error represents an MCP protocol error.
type Error struct {
	// Code is the error code.
	Code ErrorCode `json:"code"`

	// Message is the error description.
	Message string `json:"message"`

	// Data contains additional error details.
	Data any `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// IsNotFound returns true if the error indicates a resource was not found.
func (e *Error) IsNotFound() bool {
	return e.Code == ErrorCodeResourceNotFound ||
		e.Code == ErrorCodeToolNotFound ||
		e.Code == ErrorCodePromptNotFound
}

// IsRetryable returns true if the error might succeed on retry.
func (e *Error) IsRetryable() bool {
	return e.Code == ErrorCodeInternalError
}

// =============================================================================
// Connection Options
// =============================================================================

// ConnectOptions configures a server connection.
type ConnectOptions struct {
	// ProtocolVersion is the preferred protocol version.
	// Defaults to LatestProtocolVersion.
	ProtocolVersion ProtocolVersion

	// ClientInfo identifies the client to the server.
	ClientInfo *ClientInfo

	// Capabilities declares client capabilities.
	Capabilities *ClientCapabilities

	// SamplingHandler handles sampling requests from the server.
	// If nil, sampling requests will be rejected.
	SamplingHandler SamplingHandler

	// ConnectTimeout is the maximum time to establish a connection.
	ConnectTimeout time.Duration

	// RequestTimeout is the default timeout for requests.
	RequestTimeout time.Duration

	// RetryAttempts is the number of times to retry failed requests.
	RetryAttempts int

	// RetryDelay is the initial delay between retries.
	RetryDelay time.Duration
}

// DefaultConnectOptions returns sensible defaults for connection options.
func DefaultConnectOptions() *ConnectOptions {
	return &ConnectOptions{
		ProtocolVersion: LatestProtocolVersion,
		ClientInfo: &ClientInfo{
			Name:    "flip2",
			Version: "2.0.0",
		},
		Capabilities: &ClientCapabilities{
			Sampling: &SamplingCapability{},
			Roots: &RootsCapability{
				ListChanged: true,
			},
		},
		ConnectTimeout: 30 * time.Second,
		RequestTimeout: 60 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     time.Second,
	}
}
