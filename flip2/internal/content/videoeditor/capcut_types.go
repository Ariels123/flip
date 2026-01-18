// Package videoeditor - CapCut draft.content JSON schema (Phase 3)
// Implements CapCut project format for optional project generation
package videoeditor

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// CapCutProject represents a complete CapCut project (draft.content)
type CapCutProject struct {
	// Version of the CapCut project format
	Version string `json:"version"`

	// ID is a unique identifier for the project
	ID string `json:"id"`

	// Duration in microseconds
	Duration int64 `json:"duration"`

	// Materials contains all assets used in the project
	Materials CapCutMaterials `json:"materials"`

	// Tracks contains video/audio/text tracks
	Tracks []CapCutTrack `json:"tracks"`

	// CanvasConfig defines the output video properties
	CanvasConfig CapCutCanvasConfig `json:"canvas_config"`

	// Relationships between segments
	Relationships []CapCutRelationship `json:"relationships,omitempty"`

	// Extra metadata
	ExtraInfo map[string]interface{} `json:"extra_info,omitempty"`
}

// CapCutMaterials contains all assets referenced in the project
type CapCutMaterials struct {
	// Videos are video file materials
	Videos []CapCutVideoMaterial `json:"videos,omitempty"`

	// Audios are audio file materials
	Audios []CapCutAudioMaterial `json:"audios,omitempty"`

	// Texts are text materials (for text overlays)
	Texts []CapCutTextMaterial `json:"texts,omitempty"`

	// Images are image file materials
	Images []CapCutImageMaterial `json:"images,omitempty"`

	// Effects are visual effects applied
	Effects []CapCutEffectMaterial `json:"effects,omitempty"`

	// Transitions are transition effects
	Transitions []CapCutTransitionMaterial `json:"transitions,omitempty"`
}

// CapCutVideoMaterial represents a video file
type CapCutVideoMaterial struct {
	// ID is unique identifier for this material
	ID string `json:"id"`

	// Path to the video file
	Path string `json:"path"`

	// Duration in microseconds
	Duration int64 `json:"duration"`

	// Width and Height in pixels
	Width  int `json:"width"`
	Height int `json:"height"`

	// FPS (frames per second)
	FPS float64 `json:"fps"`

	// Type is always "video"
	Type string `json:"type"`

	// FileSize in bytes
	FileSize int64 `json:"file_size,omitempty"`
}

// CapCutAudioMaterial represents an audio file
type CapCutAudioMaterial struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Duration int64  `json:"duration"`
	Type     string `json:"type"` // "audio"
}

// CapCutTextMaterial represents text content
type CapCutTextMaterial struct {
	ID      string                 `json:"id"`
	Content string                 `json:"content"`
	Style   CapCutTextStyle        `json:"style"`
	Type    string                 `json:"type"` // "text"
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// CapCutTextStyle defines text styling
type CapCutTextStyle struct {
	// Font properties
	FontFamily string  `json:"font_family"`
	FontSize   int     `json:"font_size"`
	FontWeight string  `json:"font_weight"` // "normal", "bold"
	FontStyle  string  `json:"font_style"`  // "normal", "italic"
	Color      string  `json:"color"`       // RGBA hex: "#RRGGBBAA"
	Opacity    float64 `json:"opacity"`     // 0.0 to 1.0

	// Text alignment
	Align string `json:"align"` // "left", "center", "right"

	// Background
	BackgroundColor   string  `json:"background_color,omitempty"`
	BackgroundOpacity float64 `json:"background_opacity,omitempty"`

	// Shadow/Outline
	StrokeColor string  `json:"stroke_color,omitempty"`
	StrokeWidth float64 `json:"stroke_width,omitempty"`
}

// CapCutImageMaterial represents an image file
type CapCutImageMaterial struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Type     string `json:"type"` // "image"
	FileSize int64  `json:"file_size,omitempty"`
}

// CapCutEffectMaterial represents a visual effect
type CapCutEffectMaterial struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // "effect"
	EffectType string                 `json:"effect_type"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CapCutTransitionMaterial represents a transition effect
type CapCutTransitionMaterial struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"` // "transition"
	TransitionType string `json:"transition_type"`
	Duration       int64  `json:"duration"` // microseconds
}

// CapCutTrack represents a single track (video/audio/text)
type CapCutTrack struct {
	// ID of the track
	ID string `json:"id"`

	// Type: "video", "audio", "text"
	Type string `json:"type"`

	// Segments are clips/elements on this track
	Segments []CapCutSegment `json:"segments"`

	// Attribute contains track-level properties
	Attribute CapCutTrackAttribute `json:"attribute,omitempty"`
}

// CapCutTrackAttribute contains track-level properties
type CapCutTrackAttribute struct {
	// Volume for audio tracks (0.0 to 1.0)
	Volume float64 `json:"volume,omitempty"`

	// Muted flag
	Muted bool `json:"muted,omitempty"`
}

// CapCutSegment represents a clip or element on a track
type CapCutSegment struct {
	// ID of the segment
	ID string `json:"id"`

	// MaterialID references the material this segment uses
	MaterialID string `json:"material_id"`

	// TargetTimeRange defines when this segment appears (microseconds)
	TargetTimeRange CapCutTimeRange `json:"target_timerange"`

	// SourceTimeRange defines which part of the material to use (microseconds)
	SourceTimeRange CapCutTimeRange `json:"source_timerange"`

	// Speed adjustment (1.0 = normal speed)
	Speed float64 `json:"speed,omitempty"`

	// Volume for this segment (0.0 to 1.0)
	Volume float64 `json:"volume,omitempty"`

	// Transform for positioning/scaling
	Transform CapCutTransform `json:"transform,omitempty"`

	// Effects applied to this segment
	Effects []string `json:"effects,omitempty"` // List of effect IDs

	// Animations
	Animations []CapCutAnimation `json:"animations,omitempty"`
}

// CapCutTimeRange represents a time range in microseconds
type CapCutTimeRange struct {
	Start    int64 `json:"start"`
	Duration int64 `json:"duration"`
}

// CapCutTransform defines positioning, scaling, and rotation
type CapCutTransform struct {
	// Position (normalized coordinates, 0.0-1.0)
	X float64 `json:"x"`
	Y float64 `json:"y"`

	// Scale (1.0 = original size)
	ScaleX float64 `json:"scale_x"`
	ScaleY float64 `json:"scale_y"`

	// Rotation in degrees
	Rotation float64 `json:"rotation"`

	// Opacity (0.0 to 1.0)
	Opacity float64 `json:"opacity"`
}

// CapCutAnimation defines an animation effect
type CapCutAnimation struct {
	// Type: "in", "out", "loop"
	Type string `json:"type"`

	// AnimationType: "fade", "slide", "zoom", etc.
	AnimationType string `json:"animation_type"`

	// Duration in microseconds
	Duration int64 `json:"duration"`

	// Parameters specific to the animation
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CapCutCanvasConfig defines output video properties
type CapCutCanvasConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`

	// Ratio as string (e.g., "16:9", "9:16")
	Ratio string `json:"ratio"`
}

// CapCutRelationship defines relationships between segments
type CapCutRelationship struct {
	// ID of the relationship
	ID string `json:"id"`

	// Type: "transition", "effect_link", etc.
	Type string `json:"type"`

	// SourceID and TargetID reference segments
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`

	// TransitionID if this is a transition relationship
	TransitionID string `json:"transition_id,omitempty"`
}

// --- Conversion Functions ---

// ToCapCutProject converts a Template to a CapCut project
func ToCapCutProject(template *Template, variables map[string]interface{}) (*CapCutProject, error) {
	// Apply variable substitution
	vs := NewVariableSubstitutor(variables)
	processedTemplate, err := vs.SubstituteTemplate(template)
	if err != nil {
		return nil, err
	}

	// Create project ID
	projectID := generateID()

	// Calculate total duration in microseconds
	maxEndTime := int64(0)
	for _, item := range processedTemplate.Timeline {
		endTime := int64((item.StartTime + item.Duration) * 1000) // Convert ms to microseconds
		if endTime > maxEndTime {
			maxEndTime = endTime
		}
	}

	project := &CapCutProject{
		Version:  "3.0.0",
		ID:       projectID,
		Duration: maxEndTime,
		Materials: CapCutMaterials{
			Videos:      make([]CapCutVideoMaterial, 0),
			Audios:      make([]CapCutAudioMaterial, 0),
			Texts:       make([]CapCutTextMaterial, 0),
			Images:      make([]CapCutImageMaterial, 0),
			Effects:     make([]CapCutEffectMaterial, 0),
			Transitions: make([]CapCutTransitionMaterial, 0),
		},
		Tracks: make([]CapCutTrack, 0),
		CanvasConfig: CapCutCanvasConfig{
			Width:  processedTemplate.Canvas.Width,
			Height: processedTemplate.Canvas.Height,
			FPS:    processedTemplate.Canvas.FPS,
			Ratio:  calculateRatio(processedTemplate.Canvas.Width, processedTemplate.Canvas.Height),
		},
		Relationships: make([]CapCutRelationship, 0),
		ExtraInfo:     make(map[string]interface{}),
	}

	// Group timeline items by type
	videoTrack := CapCutTrack{
		ID:       generateID(),
		Type:     "video",
		Segments: make([]CapCutSegment, 0),
	}

	textTrack := CapCutTrack{
		ID:       generateID(),
		Type:     "text",
		Segments: make([]CapCutSegment, 0),
	}

	audioTrack := CapCutTrack{
		ID:       generateID(),
		Type:     "audio",
		Segments: make([]CapCutSegment, 0),
	}

	// Convert timeline items to CapCut segments
	for _, item := range processedTemplate.Timeline {
		switch item.Type {
		case "video", "image":
			materialID := generateID()
			segmentID := generateID()

			// Add material
			if item.Type == "video" {
				project.Materials.Videos = append(project.Materials.Videos, CapCutVideoMaterial{
					ID:       materialID,
					Path:     item.Path,
					Duration: int64(item.Duration * 1000), // Convert to microseconds
					Type:     "video",
				})
			} else {
				project.Materials.Images = append(project.Materials.Images, CapCutImageMaterial{
					ID:   materialID,
					Path: item.Path,
					Type: "image",
				})
			}

			// Add segment
			segment := CapCutSegment{
				ID:         segmentID,
				MaterialID: materialID,
				TargetTimeRange: CapCutTimeRange{
					Start:    int64(item.StartTime * 1000),
					Duration: int64(item.Duration * 1000),
				},
				SourceTimeRange: CapCutTimeRange{
					Start:    0,
					Duration: int64(item.Duration * 1000),
				},
				Speed:  1.0,
				Volume: item.Volume,
				Transform: CapCutTransform{
					X:        0.5,
					Y:        0.5,
					ScaleX:   1.0,
					ScaleY:   1.0,
					Rotation: 0.0,
					Opacity:  1.0,
				},
			}

			// Add transitions if present
			if item.Transitions.In != nil {
				transID := generateID()
				project.Materials.Transitions = append(project.Materials.Transitions, CapCutTransitionMaterial{
					ID:             transID,
					Name:           item.Transitions.In.Type,
					Type:           "transition",
					TransitionType: item.Transitions.In.Type,
					Duration:       int64(item.Transitions.In.Duration * 1000),
				})

				segment.Animations = append(segment.Animations, CapCutAnimation{
					Type:          "in",
					AnimationType: item.Transitions.In.Type,
					Duration:      int64(item.Transitions.In.Duration * 1000),
				})
			}

			if item.Transitions.Out != nil {
				transID := generateID()
				project.Materials.Transitions = append(project.Materials.Transitions, CapCutTransitionMaterial{
					ID:             transID,
					Name:           item.Transitions.Out.Type,
					Type:           "transition",
					TransitionType: item.Transitions.Out.Type,
					Duration:       int64(item.Transitions.Out.Duration * 1000),
				})

				segment.Animations = append(segment.Animations, CapCutAnimation{
					Type:          "out",
					AnimationType: item.Transitions.Out.Type,
					Duration:      int64(item.Transitions.Out.Duration * 1000),
				})
			}

			videoTrack.Segments = append(videoTrack.Segments, segment)

		case "text":
			materialID := generateID()
			segmentID := generateID()

			// Convert text style
			style := CapCutTextStyle{
				FontFamily: "Arial",
				FontSize:   48,
				FontWeight: "normal",
				FontStyle:  "normal",
				Color:      "#FFFFFFFF",
				Opacity:    1.0,
				Align:      "center",
			}

			if item.Style != nil {
				style.FontSize = item.Style.FontSize
				style.Color = convertColorToRGBA(item.Style.FontColor)
				style.Opacity = item.Style.Opacity
			}

			// Add material
			project.Materials.Texts = append(project.Materials.Texts, CapCutTextMaterial{
				ID:      materialID,
				Content: item.Content,
				Style:   style,
				Type:    "text",
			})

			// Add segment
			segment := CapCutSegment{
				ID:         segmentID,
				MaterialID: materialID,
				TargetTimeRange: CapCutTimeRange{
					Start:    int64(item.StartTime * 1000),
					Duration: int64(item.Duration * 1000),
				},
				Transform: CapCutTransform{
					X:        0.5,
					Y:        convertTextPositionY(item.Style),
					ScaleX:   1.0,
					ScaleY:   1.0,
					Rotation: 0.0,
					Opacity:  1.0,
				},
			}

			textTrack.Segments = append(textTrack.Segments, segment)

		case "audio":
			materialID := generateID()
			segmentID := generateID()

			// Add material
			project.Materials.Audios = append(project.Materials.Audios, CapCutAudioMaterial{
				ID:       materialID,
				Path:     item.Path,
				Duration: int64(item.Duration * 1000),
				Type:     "audio",
			})

			// Add segment
			segment := CapCutSegment{
				ID:         segmentID,
				MaterialID: materialID,
				TargetTimeRange: CapCutTimeRange{
					Start:    int64(item.StartTime * 1000),
					Duration: int64(item.Duration * 1000),
				},
				SourceTimeRange: CapCutTimeRange{
					Start:    0,
					Duration: int64(item.Duration * 1000),
				},
				Speed:  1.0,
				Volume: item.Volume,
			}

			audioTrack.Segments = append(audioTrack.Segments, segment)
		}
	}

	// Add tracks to project
	if len(videoTrack.Segments) > 0 {
		project.Tracks = append(project.Tracks, videoTrack)
	}
	if len(textTrack.Segments) > 0 {
		project.Tracks = append(project.Tracks, textTrack)
	}
	if len(audioTrack.Segments) > 0 {
		project.Tracks = append(project.Tracks, audioTrack)
	}

	return project, nil
}

// ExportCapCutProject exports a CapCut project to JSON
func ExportCapCutProject(project *CapCutProject) ([]byte, error) {
	return json.MarshalIndent(project, "", "  ")
}

// --- Helper Functions ---

func generateID() string {
	return time.Now().Format("20060102150405") + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	randomBytes := make([]byte, n)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp-based if crypto/rand fails
		for i := range b {
			b[i] = letters[(int(time.Now().UnixNano())+i)%len(letters)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = letters[int(randomBytes[i])%len(letters)]
	}
	return string(b)
}

func calculateRatio(width, height int) string {
	// Calculate GCD to simplify ratio
	gcd := func(a, b int) int {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}

	divisor := gcd(width, height)
	w := width / divisor
	h := height / divisor

	return fmt.Sprintf("%d:%d", w, h)
}

func convertColorToRGBA(color string) string {
	// Convert #RRGGBB to #RRGGBBAA
	if len(color) == 7 && color[0] == '#' {
		return color + "FF"
	}
	return color
}

func convertTextPositionY(style *TextStyle) float64 {
	if style == nil || style.Position == "" {
		return 0.5 // center
	}

	switch style.Position {
	case "top":
		return 0.1
	case "center":
		return 0.5
	case "bottom":
		return 0.9
	default:
		return 0.5
	}
}
