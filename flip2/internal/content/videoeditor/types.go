// Package videoeditor implements enhanced video editing with FFmpeg.
// Provides transitions, text overlays, color correction, and template-based rendering.
package videoeditor

import (
	"encoding/json"
)

// Input represents the data required to execute a video editing operation.
type Input struct {
	// VideoFiles contains paths to input video files
	VideoFiles []string `json:"video_files"`

	// TemplateName is the name of a template to apply (optional)
	TemplateName string `json:"template_name,omitempty"`

	// Operations are explicit video editing operations to apply
	Operations []Operation `json:"operations,omitempty"`

	// OutputFormat is the desired output format ("mp4", "mov", "webm", etc.)
	OutputFormat string `json:"output_format"`

	// Quality specifies resolution and encoding quality
	// Values: "720p", "1080p", "2k", "4k"
	Quality string `json:"quality"`

	// FPS is the frames per second for output video (default: 30)
	FPS int `json:"fps,omitempty"`

	// Metadata contains additional context (title, description, etc.)
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// OutputPath is an optional explicit output file path
	OutputPath string `json:"output_path,omitempty"`
}

// Operation represents a single video editing operation.
type Operation struct {
	// Type of operation: "cut", "concat", "transition", "text_overlay", "color_correct", "scale"
	Type string `json:"type"`

	// StartTime is when the operation begins (in milliseconds)
	StartTime float64 `json:"start_time,omitempty"`

	// Duration of the operation (in milliseconds)
	Duration float64 `json:"duration,omitempty"`

	// Parameters contains operation-specific settings
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// CutOperation specifies trimming a video clip.
type CutOperation struct {
	InputFile string  `json:"input_file"`
	StartTime float64 `json:"start_time"` // seconds
	Duration  float64 `json:"duration"`   // seconds
}

// ConcatOperation specifies concatenating multiple clips.
type ConcatOperation struct {
	InputFiles []string `json:"input_files"`
}

// TransitionOperation specifies a transition between clips.
type TransitionOperation struct {
	// Type: "fade", "dissolve", "crossfade", "wipe"
	Type     string  `json:"type"`
	Duration float64 `json:"duration"` // milliseconds
	// Additional effect-specific parameters
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// TextOverlayOperation adds text to the video.
type TextOverlayOperation struct {
	// Content is the text to display
	Content string `json:"content"`

	// StartTime when text appears (milliseconds)
	StartTime float64 `json:"start_time"`

	// Duration text is visible (milliseconds)
	Duration float64 `json:"duration"`

	// Position: "top", "center", "bottom", or coordinates "x,y"
	Position string `json:"position"`

	// FontSize in pixels
	FontSize int `json:"font_size"`

	// FontColor in hex format (#RRGGBB)
	FontColor string `json:"font_color"`

	// FontFile path to TTF font (optional)
	FontFile string `json:"font_file,omitempty"`

	// Opacity from 0.0 to 1.0
	Opacity float64 `json:"opacity"`

	// Additional parameters
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ColorCorrectionOperation applies color correction filters.
type ColorCorrectionOperation struct {
	// Brightness adjustment (-1.0 to 1.0)
	Brightness float64 `json:"brightness,omitempty"`

	// Contrast adjustment (0.5 to 2.0, 1.0 = no change)
	Contrast float64 `json:"contrast,omitempty"`

	// Saturation adjustment (0.0 to 2.0, 1.0 = no change)
	Saturation float64 `json:"saturation,omitempty"`

	// Hue rotation in degrees (-180 to 180)
	Hue float64 `json:"hue,omitempty"`

	// Gamma adjustment for overall tone
	Gamma float64 `json:"gamma,omitempty"`
}

// ScaleOperation changes video resolution.
type ScaleOperation struct {
	// Width in pixels (or -1 to maintain aspect ratio)
	Width int `json:"width"`

	// Height in pixels (or -1 to maintain aspect ratio)
	Height int `json:"height"`

	// Force: if true, scale without preserving aspect ratio
	Force bool `json:"force,omitempty"`
}

// Output represents the result of a video editing operation.
type Output struct {
	// VideoFile is the path to the output video file
	VideoFile string `json:"video_file"`

	// Duration of the output video in seconds
	Duration float64 `json:"duration"`

	// Resolution of the output (e.g., "1920x1080")
	Resolution string `json:"resolution"`

	// FileSize in bytes
	FileSize int64 `json:"file_size"`

	// FrameRate (FPS) of the output
	FrameRate int `json:"frame_rate"`

	// Codec information
	VideoCodec string `json:"video_codec,omitempty"`
	AudioCodec string `json:"audio_codec,omitempty"`

	// Metadata about the rendering
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// ProcessingTime in seconds
	ProcessingTime float64 `json:"processing_time"`
}

// QualityPreset defines encoding parameters for common resolutions.
type QualityPreset struct {
	Name      string // "720p", "1080p", "2k", "4k"
	Width     int
	Height    int
	Bitrate   string // "2500k", "5000k", etc.
	MaxBitrate string
	BufSize   string
}

// QualityPresets maps quality names to their FFmpeg parameters.
var QualityPresets = map[string]QualityPreset{
	"720p": {
		Name:       "720p",
		Width:      1280,
		Height:     720,
		Bitrate:    "2500k",
		MaxBitrate: "3000k",
		BufSize:    "6000k",
	},
	"1080p": {
		Name:       "1080p",
		Width:      1920,
		Height:     1080,
		Bitrate:    "5000k",
		MaxBitrate: "6000k",
		BufSize:    "12000k",
	},
	"2k": {
		Name:       "2k",
		Width:      2560,
		Height:     1440,
		Bitrate:    "8000k",
		MaxBitrate: "10000k",
		BufSize:    "20000k",
	},
	"4k": {
		Name:       "4k",
		Width:      3840,
		Height:     2160,
		Bitrate:    "15000k",
		MaxBitrate: "18000k",
		BufSize:    "36000k",
	},
}

// TransitionTypes defines available transition effects.
var TransitionTypes = []string{
	"fade",      // Fade in/out
	"dissolve",  // Cross-dissolve
	"crossfade", // Audio crossfade
	"wipe",      // Directional wipe
	"slidedown", // Slide transition
}

// TextPositions defines common text positioning.
var TextPositions = map[string]string{
	"top":    "x=(w-text_w)/2:y=10",
	"center": "x=(w-text_w)/2:y=(h-text_h)/2",
	"bottom": "x=(w-text_w)/2:y=h-text_h-10",
}
