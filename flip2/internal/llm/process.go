package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"flip2/internal/errors"
)

// ProcessBackend executes AI CLI tools via stdin/stdout.
//
// This is the primary backend for local CLI tools like 'claude' and 'gemini'.
// It uses subprocess execution with pipes, which is efficient because:
//   - No network overhead
//   - Native streaming via stdout
//   - CLI tools handle their own auth and retries
//   - Process isolation provides crash protection
type ProcessBackend struct {
	name         string
	command      string
	defaultArgs  []string
	models       []string
	defaultModel string
	modelConfigs map[string]ModelConfig
	timeout      time.Duration
	logger       *slog.Logger

	mu             sync.RWMutex
	execCount      int64
	lastExec       time.Time
	quotaExhausted bool
	quotaResetsAt  time.Time
}

// ProcessBackendConfig holds configuration for ProcessBackend.
type ProcessBackendConfig struct {
	Name         string
	Command      string
	DefaultArgs  []string
	Models       []string
	DefaultModel string
	ModelConfigs map[string]ModelConfig
	Timeout      time.Duration
	Logger       *slog.Logger
}

// NewProcessBackend creates a ProcessBackend with the given configuration.
func NewProcessBackend(cfg ProcessBackendConfig) *ProcessBackend {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	logger := cfg.Logger
	if logger == nil {
		// Use a default noop logger if none provided
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &ProcessBackend{
		name:         cfg.Name,
		command:      cfg.Command,
		defaultArgs:  cfg.DefaultArgs,
		models:       cfg.Models,
		defaultModel: cfg.DefaultModel,
		modelConfigs: cfg.ModelConfigs,
		timeout:      timeout,
		logger:       logger,
	}
}

// NewClaudeBackend creates a ProcessBackend configured for the Claude CLI.
func NewClaudeBackend() *ProcessBackend {
	return NewProcessBackend(ProcessBackendConfig{
		Name:         "claude",
		Command:      "claude",
		DefaultArgs:  []string{"-p"},
		Models:       []string{"claude-sonnet-4", "claude-opus-4", "claude-3-5-sonnet", "claude-3-5-haiku"},
		DefaultModel: "claude-sonnet-4",
		ModelConfigs: map[string]ModelConfig{
			"claude-sonnet-4":   {ID: "claude-sonnet-4-20250514", InputCostPer1M: 3.0, OutputCostPer1M: 15.0, MaxTokens: 8192},
			"claude-opus-4":     {ID: "claude-opus-4-20250514", InputCostPer1M: 15.0, OutputCostPer1M: 75.0, MaxTokens: 4096},
			"claude-3-5-sonnet": {ID: "claude-3-5-sonnet-20241022", InputCostPer1M: 3.0, OutputCostPer1M: 15.0, MaxTokens: 8192},
			"claude-3-5-haiku":  {ID: "claude-3-5-haiku-20241022", InputCostPer1M: 0.25, OutputCostPer1M: 1.25, MaxTokens: 8192},
		},
		Timeout: 5 * time.Minute,
	})
}

// NewClaudeCodeBackend creates a ProcessBackend for Claude with --dangerously-skip-permissions.
func NewClaudeCodeBackend() *ProcessBackend {
	return NewProcessBackend(ProcessBackendConfig{
		Name:         "claude-code",
		Command:      "claude",
		DefaultArgs:  []string{"--dangerously-skip-permissions", "-p", "--output-format", "text"},
		Models:       []string{"claude-sonnet-4", "claude-opus-4"},
		DefaultModel: "claude-sonnet-4",
		ModelConfigs: map[string]ModelConfig{
			"claude-sonnet-4": {ID: "claude-sonnet-4-20250514", InputCostPer1M: 3.0, OutputCostPer1M: 15.0, MaxTokens: 8192},
			"claude-opus-4":   {ID: "claude-opus-4-20250514", InputCostPer1M: 15.0, OutputCostPer1M: 75.0, MaxTokens: 4096},
		},
		Timeout: 10 * time.Minute,
	})
}

// NewGeminiBackend creates a ProcessBackend configured for the Gemini CLI.
func NewGeminiBackend() *ProcessBackend {
	return NewProcessBackend(ProcessBackendConfig{
		Name:         "gemini",
		Command:      "gemini",
		DefaultArgs:  []string{"-y", "--output-format", "text"},
		Models:       []string{"gemini-2.5-flash", "gemini-2.5-pro"},
		DefaultModel: "gemini-2.5-flash",
		ModelConfigs: map[string]ModelConfig{
			"gemini-2.5-flash": {ID: "gemini-2.5-flash", InputCostPer1M: 0.075, OutputCostPer1M: 0.30, MaxTokens: 8192},
			"gemini-2.5-pro":   {ID: "gemini-2.5-pro", InputCostPer1M: 1.25, OutputCostPer1M: 10.0, MaxTokens: 8192},
		},
		Timeout: 5 * time.Minute,
	})
}

// NewAntigravityBackend creates a ProcessBackend for Gemini with human-in-loop capabilities.
func NewAntigravityBackend() *ProcessBackend {
	return NewProcessBackend(ProcessBackendConfig{
		Name:         "antigravity",
		Command:      "gemini",
		DefaultArgs:  []string{"-m", "gemini-2.5-pro", "-y", "--output-format", "text"},
		Models:       []string{"gemini-2.5-pro"},
		DefaultModel: "gemini-2.5-pro",
		ModelConfigs: map[string]ModelConfig{
			"gemini-2.5-pro": {ID: "gemini-2.5-pro", InputCostPer1M: 1.25, OutputCostPer1M: 10.0, MaxTokens: 8192},
		},
		Timeout: 10 * time.Minute,
	})
}

// Name returns the backend identifier.
func (p *ProcessBackend) Name() string {
	return p.name
}

// Models returns the available models.
func (p *ProcessBackend) Models() []string {
	return p.models
}

// DefaultModel returns the default model.
func (p *ProcessBackend) DefaultModel() string {
	return p.defaultModel
}

// Execute runs the CLI with the given prompt and returns the complete response.
func (p *ProcessBackend) Execute(ctx context.Context, prompt string, opts *Options) (*Response, error) {
	start := time.Now()

	// Build command arguments
	args := p.buildArgs(opts)

	// Append prompt as positional argument
	args = append(args, prompt)

	// Apply timeout from options or default
	timeout := p.timeout
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Log execution start
	p.logger.InfoContext(ctx, "executing command",
		"backend", p.name,
		"command", p.command,
		"model", p.getModel(opts),
		"timeout_ms", timeout.Milliseconds(),
	)

	// Create command
	cmd := exec.CommandContext(execCtx, p.command, args...)

	// Set up pipes
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("failed to start %s", p.command)
		p.logger.ErrorContext(ctx, errMsg,
			"backend", p.name,
			"command", p.command,
			"error", err.Error(),
		)
		return nil, errors.NewExecutionFailed(errMsg, true, err)
	}

	// Wait for command to finish
	err := cmd.Wait()
	durationMs := time.Since(start).Milliseconds()

	// Check for context cancellation
	if execCtx.Err() == context.DeadlineExceeded {
		p.logger.WarnContext(ctx, "execution timeout exceeded",
			"backend", p.name,
			"command", p.command,
			"timeout_ms", timeout.Milliseconds(),
			"duration_ms", durationMs,
		)
		return nil, errors.NewTimeout(timeout, execCtx.Err())
	}
	if execCtx.Err() == context.Canceled {
		p.logger.InfoContext(ctx, "execution canceled",
			"backend", p.name,
			"command", p.command,
			"duration_ms", durationMs,
		)
		return nil, errors.NewCanceled("execution was canceled")
	}

	// Check for quota exhaustion in stderr
	if p.isQuotaError(stderr.String()) {
		p.logger.WarnContext(ctx, "quota exhausted",
			"backend", p.name,
			"command", p.command,
			"duration_ms", durationMs,
		)
		p.markQuotaExhausted()
		return nil, errors.NewQuotaExhausted("quota exhausted", time.Now().Add(1*time.Hour))
	}

	// Check for other errors
	if err != nil {
		p.logger.ErrorContext(ctx, "command execution failed",
			"backend", p.name,
			"command", p.command,
			"error", err.Error(),
			"duration_ms", durationMs,
			"stderr_length", len(stderr.String()),
		)
		return nil, errors.NewExecutionFailed(fmt.Sprintf("%s error: %s", p.command, stderr.String()), false, err)
	}

	// Parse response
	content := strings.TrimSpace(stdout.String())
	model := p.getModel(opts)
	inputTokens, outputTokens := p.estimateTokens(prompt, content)
	cost := p.calculateCost(model, inputTokens, outputTokens)

	// Log successful execution
	p.logger.InfoContext(ctx, "command execution completed",
		"backend", p.name,
		"command", p.command,
		"model", model,
		"duration_ms", durationMs,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"cost_usd", fmt.Sprintf("%.6f", cost),
	)

	// Update metrics
	p.mu.Lock()
	p.execCount++
	p.lastExec = time.Now()
	p.mu.Unlock()

	return &Response{
		Content:      content,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      cost,
		FinishReason: "stop",
		Latency:      time.Since(start),
		Stderr:       stderr.String(),
	}, nil
}

// Stream runs the CLI and streams output chunks as they arrive.
func (p *ProcessBackend) Stream(ctx context.Context, prompt string, opts *Options) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	// Build command arguments
	args := p.buildArgs(opts)
	args = append(args, prompt)

	// Apply timeout
	timeout := p.timeout
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Log streaming start
	p.logger.InfoContext(ctx, "starting stream",
		"backend", p.name,
		"command", p.command,
		"model", p.getModel(opts),
		"timeout_ms", timeout.Milliseconds(),
	)

	// Create command
	cmd := exec.CommandContext(execCtx, p.command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		errMsg := "failed to create stdout pipe"
		p.logger.ErrorContext(ctx, errMsg,
			"backend", p.name,
			"command", p.command,
			"error", err.Error(),
		)
		return nil, errors.NewExecutionFailed(errMsg, true, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		errMsg := "failed to create stderr pipe"
		p.logger.ErrorContext(ctx, errMsg,
			"backend", p.name,
			"command", p.command,
			"error", err.Error(),
		)
		return nil, errors.NewExecutionFailed(errMsg, true, err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		errMsg := fmt.Sprintf("failed to start %s", p.command)
		p.logger.ErrorContext(ctx, errMsg,
			"backend", p.name,
			"command", p.command,
			"error", err.Error(),
		)
		return nil, errors.NewExecutionFailed(errMsg, true, err)
	}

	go func() {
		streamStartTime := time.Now()
		defer close(ch)
		defer cancel()

		// Capture stderr for error checking
		var stderrBuf bytes.Buffer
		go io.Copy(&stderrBuf, stderr)

		// Stream stdout
		var fullContent strings.Builder
		reader := bufio.NewReader(stdout)

		for {
			// Read character by character for true streaming
			char, _, err := reader.ReadRune()
			if err != nil {
				if err != io.EOF {
					p.logger.DebugContext(ctx, "stream read error",
						"backend", p.name,
						"error", err.Error(),
					)
					ch <- StreamChunk{Error: err, Done: true}
				}
				break
			}

			text := string(char)
			fullContent.WriteString(text)

			select {
			case ch <- StreamChunk{Text: text}:
			case <-execCtx.Done():
				durationMs := time.Since(streamStartTime).Milliseconds()
				p.logger.WarnContext(ctx, "stream context canceled",
					"backend", p.name,
					"command", p.command,
					"duration_ms", durationMs,
					"bytes_streamed", len(fullContent.String()),
				)
				ch <- StreamChunk{Error: execCtx.Err(), Done: true}
				return
			}
		}

		// Wait for command to finish
		cmdErr := cmd.Wait()
		durationMs := time.Since(streamStartTime).Milliseconds()

		// Check for errors
		if p.isQuotaError(stderrBuf.String()) {
			p.logger.WarnContext(ctx, "quota exhausted during stream",
				"backend", p.name,
				"command", p.command,
				"duration_ms", durationMs,
			)
			p.markQuotaExhausted()
			ch <- StreamChunk{Error: errors.NewQuotaExhausted("quota exhausted", time.Now().Add(1*time.Hour)), Done: true}
			return
		}

		if cmdErr != nil && execCtx.Err() == nil {
			p.logger.ErrorContext(ctx, "stream command failed",
				"backend", p.name,
				"command", p.command,
				"error", cmdErr.Error(),
				"duration_ms", durationMs,
				"stderr_length", len(stderrBuf.String()),
			)
			ch <- StreamChunk{Error: errors.NewExecutionFailed(fmt.Sprintf("%s error", p.command), false, cmdErr), Done: true}
			return
		}

		// Final chunk with token counts
		content := fullContent.String()
		inputTokens, outputTokens := p.estimateTokens(prompt, content)

		// Log successful stream completion
		p.logger.InfoContext(ctx, "stream completed",
			"backend", p.name,
			"command", p.command,
			"duration_ms", durationMs,
			"input_tokens", inputTokens,
			"output_tokens", outputTokens,
			"content_length", len(content),
		)

		ch <- StreamChunk{
			Done:         true,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}

		// Update metrics
		p.mu.Lock()
		p.execCount++
		p.lastExec = time.Now()
		p.mu.Unlock()
	}()

	return ch, nil
}

// CheckQuota returns current quota availability (0-1 scale, -1 if unknown).
func (p *ProcessBackend) CheckQuota(ctx context.Context) (float64, error) {
	p.mu.RLock()
	exhausted := p.quotaExhausted
	resetsAt := p.quotaResetsAt
	p.mu.RUnlock()

	if exhausted {
		if time.Now().After(resetsAt) {
			p.mu.Lock()
			p.quotaExhausted = false
			p.mu.Unlock()
			return 1.0, nil
		}
		return 0.0, nil
	}

	return 1.0, nil
}

// IsAvailable checks if the backend CLI is accessible.
func (p *ProcessBackend) IsAvailable(ctx context.Context) bool {
	// Check if command exists
	_, err := exec.LookPath(p.command)
	if err != nil {
		return false
	}

	// Check quota
	p.mu.RLock()
	exhausted := p.quotaExhausted
	resetsAt := p.quotaResetsAt
	p.mu.RUnlock()

	if exhausted && time.Now().Before(resetsAt) {
		return false
	}

	return true
}

// buildArgs constructs command line arguments from options.
func (p *ProcessBackend) buildArgs(opts *Options) []string {
	args := make([]string, len(p.defaultArgs))
	copy(args, p.defaultArgs)

	if opts == nil {
		return args
	}

	// Add model flag if specified and different from default
	if opts.Model != "" && opts.Model != p.defaultModel {
		switch p.name {
		case "claude", "claude-code":
			args = append(args, "--model", opts.Model)
		case "gemini", "antigravity":
			args = append(args, "-m", opts.Model)
		}
	}

	// Add system prompt if specified
	if opts.SystemPrompt != "" {
		switch p.name {
		case "claude", "claude-code":
			args = append(args, "--system", opts.SystemPrompt)
		}
	}

	// Add max tokens if specified
	if opts.MaxTokens > 0 {
		switch p.name {
		case "claude", "claude-code":
			args = append(args, "--max-tokens", strconv.Itoa(opts.MaxTokens))
		}
	}

	return args
}

// getModel returns the model to use from options or default.
func (p *ProcessBackend) getModel(opts *Options) string {
	if opts != nil && opts.Model != "" {
		return opts.Model
	}
	return p.defaultModel
}

// estimateTokens provides a rough token count estimate.
// Rule of thumb: ~4 characters per token for English text.
func (p *ProcessBackend) estimateTokens(input, output string) (int, int) {
	inputTokens := len(input) / 4
	if inputTokens == 0 {
		inputTokens = 1
	}
	outputTokens := len(output) / 4
	if outputTokens == 0 {
		outputTokens = 1
	}
	return inputTokens, outputTokens
}

// calculateCost computes the cost in USD based on model pricing.
func (p *ProcessBackend) calculateCost(model string, inputTokens, outputTokens int) float64 {
	cfg, ok := p.modelConfigs[model]
	if !ok {
		return 0
	}

	inputCost := float64(inputTokens) * cfg.InputCostPer1M / 1_000_000
	outputCost := float64(outputTokens) * cfg.OutputCostPer1M / 1_000_000
	return inputCost + outputCost
}

// isQuotaError checks if the error message indicates quota exhaustion.
func (p *ProcessBackend) isQuotaError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	quotaPatterns := []string{
		"quota", "exhausted", "rate limit", "429",
		"too many requests", "limit exceeded",
	}
	for _, pattern := range quotaPatterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}
	return false
}

// markQuotaExhausted sets the quota state to exhausted.
func (p *ProcessBackend) markQuotaExhausted() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quotaExhausted = true
	// Default reset time is 1 hour
	resetDuration := 1 * time.Hour
	if p.name == "antigravity" {
		resetDuration = 3 * time.Hour
	}
	p.quotaResetsAt = time.Now().Add(resetDuration)
}

// GetMetrics returns execution metrics for monitoring.
func (p *ProcessBackend) GetMetrics() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]any{
		"name":            p.name,
		"exec_count":      p.execCount,
		"last_exec":       p.lastExec,
		"quota_exhausted": p.quotaExhausted,
		"quota_resets_at": p.quotaResetsAt,
	}
}

// ParseTokenCounts attempts to extract token counts from CLI output.
func ParseTokenCounts(output string) (inputTokens, outputTokens int, found bool) {
	// Try JSON format: {"input_tokens": 100, "output_tokens": 50}
	var jsonData struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
	if err := json.Unmarshal([]byte(output), &jsonData); err == nil {
		if jsonData.InputTokens > 0 || jsonData.OutputTokens > 0 {
			return jsonData.InputTokens, jsonData.OutputTokens, true
		}
	}

	// Try pattern: "Tokens: 100 input, 50 output"
	re := regexp.MustCompile(`(\d+)\s*input.*?(\d+)\s*output`)
	if matches := re.FindStringSubmatch(output); len(matches) == 3 {
		input, _ := strconv.Atoi(matches[1])
		out, _ := strconv.Atoi(matches[2])
		return input, out, true
	}

	return 0, 0, false
}

// CommandExists checks if a CLI command is available in PATH.
func CommandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// GetAvailableBackends returns backends whose CLI tools are installed.
func GetAvailableBackends() []string {
	available := []string{}
	commands := map[string]string{
		"claude":      "claude",
		"claude-code": "claude",
		"gemini":      "gemini",
		"antigravity": "gemini",
	}

	for name, cmd := range commands {
		if CommandExists(cmd) {
			available = append(available, name)
		}
	}

	// Check for environment variables that indicate API availability
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		// Claude API available
	}
	if os.Getenv("GOOGLE_API_KEY") != "" || os.Getenv("GEMINI_API_KEY") != "" {
		// Gemini API available
	}

	return available
}

// DefaultRegistry creates a registry with all standard backends.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Register backends if their CLI tools are available
	if CommandExists("claude") {
		r.Register(NewClaudeBackend())
		r.Register(NewClaudeCodeBackend())
	}

	if CommandExists("gemini") {
		r.Register(NewGeminiBackend())
		r.Register(NewAntigravityBackend())
	}

	return r
}
