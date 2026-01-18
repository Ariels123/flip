// Package videoeditor - Effect plugin system for extensible video effects.
package videoeditor

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// EffectPlugin defines the interface for video effect plugins.
type EffectPlugin interface {
	// Name returns the unique identifier for this effect
	Name() string

	// Description returns a human-readable description
	Description() string

	// Apply applies the effect to the input video and produces output
	Apply(ctx context.Context, input EffectInput) (EffectOutput, error)

	// ValidateParameters checks if the provided parameters are valid
	ValidateParameters(params map[string]interface{}) error
}

// EffectInput contains the input data for an effect.
type EffectInput struct {
	// InputPath is the path to the input video file
	InputPath string

	// OutputPath is the desired output file path
	OutputPath string

	// Parameters are effect-specific settings
	Parameters map[string]interface{}

	// FFmpegPath is the path to the ffmpeg binary
	FFmpegPath string

	// FFprobePath is the path to the ffprobe binary
	FFprobePath string
}

// EffectOutput contains the result of applying an effect.
type EffectOutput struct {
	// OutputPath is the path to the output video
	OutputPath string

	// Metadata contains additional information about the effect application
	Metadata map[string]interface{}
}

// EffectRegistry manages registered effect plugins.
type EffectRegistry struct {
	mu      sync.RWMutex
	effects map[string]EffectPlugin
}

// NewEffectRegistry creates a new effect registry.
func NewEffectRegistry() *EffectRegistry {
	registry := &EffectRegistry{
		effects: make(map[string]EffectPlugin),
	}

	// Register built-in effects
	registry.RegisterBuiltinEffects()

	return registry
}

// Register registers an effect plugin.
func (r *EffectRegistry) Register(plugin EffectPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := plugin.Name()
	if _, exists := r.effects[name]; exists {
		return fmt.Errorf("effect already registered: %s", name)
	}

	r.effects[name] = plugin
	return nil
}

// Get retrieves an effect plugin by name.
func (r *EffectRegistry) Get(name string) (EffectPlugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.effects[name]
	if !exists {
		return nil, fmt.Errorf("effect not found: %s", name)
	}

	return plugin, nil
}

// List returns all registered effect names.
func (r *EffectRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.effects))
	for name := range r.effects {
		names = append(names, name)
	}

	return names
}

// Apply applies an effect using the registry.
func (r *EffectRegistry) Apply(ctx context.Context, effectName string, input EffectInput) (EffectOutput, error) {
	plugin, err := r.Get(effectName)
	if err != nil {
		return EffectOutput{}, err
	}

	// Validate parameters
	if err := plugin.ValidateParameters(input.Parameters); err != nil {
		return EffectOutput{}, fmt.Errorf("invalid parameters for %s: %w", effectName, err)
	}

	// Apply effect
	return plugin.Apply(ctx, input)
}

// RegisterBuiltinEffects registers the built-in effect plugins.
func (r *EffectRegistry) RegisterBuiltinEffects() {
	// Register all built-in effects
	r.Register(&ColorCorrectEffect{})
	r.Register(&BlurEffect{})
	r.Register(&SharpenEffect{})
	r.Register(&NoiseEffect{})
	r.Register(&VignetteEffect{})
}

// Global effect registry instance
var globalRegistry *EffectRegistry
var registryOnce sync.Once

// GlobalRegistry returns the global effect registry.
func GlobalRegistry() *EffectRegistry {
	registryOnce.Do(func() {
		globalRegistry = NewEffectRegistry()
	})
	return globalRegistry
}

// --- Built-in Effect Implementations ---

// ColorCorrectEffect implements color correction.
type ColorCorrectEffect struct{}

func (e *ColorCorrectEffect) Name() string {
	return "color_correct"
}

func (e *ColorCorrectEffect) Description() string {
	return "Adjusts brightness, contrast, saturation, hue, and gamma"
}

func (e *ColorCorrectEffect) ValidateParameters(params map[string]interface{}) error {
	// All parameters are optional with defaults
	return nil
}

func (e *ColorCorrectEffect) Apply(ctx context.Context, input EffectInput) (EffectOutput, error) {
	// Build FFmpeg filter for color correction
	filters := make([]string, 0)

	// Extract parameters with defaults
	brightness := getFloat64Param(input.Parameters, "brightness", 0.0)
	contrast := getFloat64Param(input.Parameters, "contrast", 1.0)
	saturation := getFloat64Param(input.Parameters, "saturation", 1.0)
	gamma := getFloat64Param(input.Parameters, "gamma", 1.0)

	// Build eq filter
	filters = append(filters, fmt.Sprintf("eq=brightness=%.2f:contrast=%.2f:saturation=%.2f:gamma=%.2f",
		brightness, contrast, saturation, gamma))

	// Hue adjustment
	hue := getFloat64Param(input.Parameters, "hue", 0.0)
	if hue != 0.0 {
		filters = append(filters, fmt.Sprintf("hue=h=%.0f", hue))
	}

	// Combine filters
	filterChain := filters[0]
	if len(filters) > 1 {
		filterChain = filters[0] + "," + filters[1]
	}

	// Execute FFmpeg
	cmd := NewFFmpegCommand(input.FFmpegPath, input.FFprobePath)
	cmd.AddInput(input.InputPath)
	cmd.AddArg("-vf").AddArg(filterChain)
	cmd.AddArg("-c:a").AddArg("copy")
	cmd.SetOutput(input.OutputPath)

	execCmd := exec.CommandContext(ctx, input.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return EffectOutput{}, fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return EffectOutput{
		OutputPath: input.OutputPath,
		Metadata: map[string]interface{}{
			"brightness": brightness,
			"contrast":   contrast,
			"saturation": saturation,
			"gamma":      gamma,
			"hue":        hue,
		},
	}, nil
}

// BlurEffect implements blur effect.
type BlurEffect struct{}

func (e *BlurEffect) Name() string {
	return "blur"
}

func (e *BlurEffect) Description() string {
	return "Applies Gaussian blur to the video"
}

func (e *BlurEffect) ValidateParameters(params map[string]interface{}) error {
	if strength, ok := params["strength"]; ok {
		if s, ok := strength.(float64); ok {
			if s < 0 || s > 100 {
				return fmt.Errorf("blur strength must be between 0 and 100")
			}
		}
	}
	return nil
}

func (e *BlurEffect) Apply(ctx context.Context, input EffectInput) (EffectOutput, error) {
	strength := getFloat64Param(input.Parameters, "strength", 5.0)

	// Convert strength to sigma (0-100 -> 0-10)
	sigma := strength / 10.0

	cmd := NewFFmpegCommand(input.FFmpegPath, input.FFprobePath)
	cmd.AddInput(input.InputPath)
	cmd.AddArg("-vf").AddArg(fmt.Sprintf("gblur=sigma=%.2f", sigma))
	cmd.AddArg("-c:a").AddArg("copy")
	cmd.SetOutput(input.OutputPath)

	execCmd := exec.CommandContext(ctx, input.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return EffectOutput{}, fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return EffectOutput{
		OutputPath: input.OutputPath,
		Metadata: map[string]interface{}{
			"strength": strength,
			"sigma":    sigma,
		},
	}, nil
}

// SharpenEffect implements sharpening effect.
type SharpenEffect struct{}

func (e *SharpenEffect) Name() string {
	return "sharpen"
}

func (e *SharpenEffect) Description() string {
	return "Sharpens the video using unsharp mask"
}

func (e *SharpenEffect) ValidateParameters(params map[string]interface{}) error {
	if strength, ok := params["strength"]; ok {
		if s, ok := strength.(float64); ok {
			if s < 0 || s > 100 {
				return fmt.Errorf("sharpen strength must be between 0 and 100")
			}
		}
	}
	return nil
}

func (e *SharpenEffect) Apply(ctx context.Context, input EffectInput) (EffectOutput, error) {
	strength := getFloat64Param(input.Parameters, "strength", 50.0)

	// Convert strength to unsharp parameters (0-100 -> 0-5)
	amount := strength / 20.0

	cmd := NewFFmpegCommand(input.FFmpegPath, input.FFprobePath)
	cmd.AddInput(input.InputPath)
	cmd.AddArg("-vf").AddArg(fmt.Sprintf("unsharp=5:5:%.2f:5:5:%.2f", amount, amount))
	cmd.AddArg("-c:a").AddArg("copy")
	cmd.SetOutput(input.OutputPath)

	execCmd := exec.CommandContext(ctx, input.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return EffectOutput{}, fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return EffectOutput{
		OutputPath: input.OutputPath,
		Metadata: map[string]interface{}{
			"strength": strength,
			"amount":   amount,
		},
	}, nil
}

// NoiseEffect implements film grain/noise effect.
type NoiseEffect struct{}

func (e *NoiseEffect) Name() string {
	return "noise"
}

func (e *NoiseEffect) Description() string {
	return "Adds film grain or noise to the video"
}

func (e *NoiseEffect) ValidateParameters(params map[string]interface{}) error {
	if strength, ok := params["strength"]; ok {
		if s, ok := strength.(float64); ok {
			if s < 0 || s > 100 {
				return fmt.Errorf("noise strength must be between 0 and 100")
			}
		}
	}
	return nil
}

func (e *NoiseEffect) Apply(ctx context.Context, input EffectInput) (EffectOutput, error) {
	strength := getFloat64Param(input.Parameters, "strength", 20.0)

	// Convert strength to noise amount (0-100 -> 0-100)
	cmd := NewFFmpegCommand(input.FFmpegPath, input.FFprobePath)
	cmd.AddInput(input.InputPath)
	cmd.AddArg("-vf").AddArg(fmt.Sprintf("noise=alls=%d:allf=t+u", int(strength)))
	cmd.AddArg("-c:a").AddArg("copy")
	cmd.SetOutput(input.OutputPath)

	execCmd := exec.CommandContext(ctx, input.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return EffectOutput{}, fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return EffectOutput{
		OutputPath: input.OutputPath,
		Metadata: map[string]interface{}{
			"strength": strength,
		},
	}, nil
}

// VignetteEffect implements vignette effect.
type VignetteEffect struct{}

func (e *VignetteEffect) Name() string {
	return "vignette"
}

func (e *VignetteEffect) Description() string {
	return "Adds a vignette (darkened edges) to the video"
}

func (e *VignetteEffect) ValidateParameters(params map[string]interface{}) error {
	if strength, ok := params["strength"]; ok {
		if s, ok := strength.(float64); ok {
			if s < 0 || s > 100 {
				return fmt.Errorf("vignette strength must be between 0 and 100")
			}
		}
	}
	return nil
}

func (e *VignetteEffect) Apply(ctx context.Context, input EffectInput) (EffectOutput, error) {
	strength := getFloat64Param(input.Parameters, "strength", 50.0)

	// Convert strength to vignette parameters
	// angle controls how much of the edge is darkened (0-PI/2)
	angle := fmt.Sprintf("PI/%.1f", 4.0-(strength/50.0))

	cmd := NewFFmpegCommand(input.FFmpegPath, input.FFprobePath)
	cmd.AddInput(input.InputPath)
	cmd.AddArg("-vf").AddArg(fmt.Sprintf("vignette=angle=%s", angle))
	cmd.AddArg("-c:a").AddArg("copy")
	cmd.SetOutput(input.OutputPath)

	execCmd := exec.CommandContext(ctx, input.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return EffectOutput{}, fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return EffectOutput{
		OutputPath: input.OutputPath,
		Metadata: map[string]interface{}{
			"strength": strength,
			"angle":    angle,
		},
	}, nil
}

// --- Helper functions ---

// getFloat64Param extracts a float64 parameter with a default value.
func getFloat64Param(params map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultValue
}

// getStringParam extracts a string parameter with a default value.
func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}

// getIntParam extracts an int parameter with a default value.
func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultValue
}

// getBoolParam extracts a bool parameter with a default value.
func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}
