// Package videoeditor - CapCut project exporter (Phase 3)
// Handles packaging and exporting CapCut projects
package videoeditor

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"flip2/internal/logger"
)

// CapCutExporter handles exporting templates to CapCut projects
type CapCutExporter struct {
	templateManager *TemplateManager
	outputDir       string
	logger          *logger.Logger
}

// NewCapCutExporter creates a new CapCut exporter
func NewCapCutExporter(tm *TemplateManager, outputDir string, log *logger.Logger) *CapCutExporter {
	return &CapCutExporter{
		templateManager: tm,
		outputDir:       outputDir,
		logger:          log,
	}
}

// ExportOptions configures the export process
type ExportOptions struct {
	// TemplateName is the template to export
	TemplateName string

	// Variables for template substitution
	Variables map[string]interface{}

	// OutputPath is the path for the exported project
	OutputPath string

	// IncludeAssets controls whether to package media files
	IncludeAssets bool

	// PackageFormat: "directory" or "zip"
	PackageFormat string
}

// ExportResult contains the export results
type ExportResult struct {
	// ProjectPath is the path to the CapCut project directory or zip file
	ProjectPath string

	// DraftContentPath is the path to draft.content file
	DraftContentPath string

	// AssetCount is the number of assets included
	AssetCount int

	// PackageSize in bytes
	PackageSize int64
}

// Export exports a template to a CapCut project
func (ce *CapCutExporter) Export(ctx context.Context, options ExportOptions) (*ExportResult, error) {
	if ce.logger != nil {
		ce.logger.InfoCtx(ctx, "Starting CapCut export",
			"template", options.TemplateName,
			"include_assets", options.IncludeAssets)
	}

	// Load template
	template, err := ce.templateManager.Load(ctx, options.TemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	// Merge template variables with provided variables (provided variables take precedence)
	mergedVariables := make(map[string]interface{})
	// Start with template defaults
	for k, v := range template.Variables {
		mergedVariables[k] = v
	}
	// Override with provided variables
	for k, v := range options.Variables {
		mergedVariables[k] = v
	}

	// Convert to CapCut project with merged variables
	project, err := ToCapCutProject(template, mergedVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to CapCut project: %w", err)
	}

	// Create output directory
	projectDir := options.OutputPath
	if projectDir == "" {
		projectDir = filepath.Join(ce.outputDir, fmt.Sprintf("capcut_project_%s", project.ID))
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	// Write draft.content file
	draftContentPath := filepath.Join(projectDir, "draft.content")
	projectJSON, err := ExportCapCutProject(project)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize project: %w", err)
	}

	if err := os.WriteFile(draftContentPath, projectJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write draft.content: %w", err)
	}

	result := &ExportResult{
		ProjectPath:      projectDir,
		DraftContentPath: draftContentPath,
		AssetCount:       0,
		PackageSize:      int64(len(projectJSON)),
	}

	// Package assets if requested
	if options.IncludeAssets {
		assetCount, totalSize, err := ce.packageAssets(ctx, project, projectDir)
		if err != nil {
			return nil, fmt.Errorf("failed to package assets: %w", err)
		}

		result.AssetCount = assetCount
		result.PackageSize += totalSize
	}

	// Create zip if requested
	if options.PackageFormat == "zip" {
		zipPath := projectDir + ".zip"
		if err := ce.createZipPackage(projectDir, zipPath); err != nil {
			return nil, fmt.Errorf("failed to create zip package: %w", err)
		}

		// Clean up directory
		os.RemoveAll(projectDir)

		result.ProjectPath = zipPath
		result.DraftContentPath = ""
	}

	if ce.logger != nil {
		ce.logger.InfoCtx(ctx, "CapCut export completed",
			"project_path", result.ProjectPath,
			"assets", result.AssetCount,
			"size_mb", float64(result.PackageSize)/(1024*1024))
	}

	return result, nil
}

// packageAssets copies all referenced assets into the project directory
func (ce *CapCutExporter) packageAssets(ctx context.Context, project *CapCutProject, projectDir string) (int, int64, error) {
	assetsDir := filepath.Join(projectDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create assets directory: %w", err)
	}

	assetCount := 0
	totalSize := int64(0)

	// Package videos
	for i, video := range project.Materials.Videos {
		if video.Path == "" {
			continue
		}

		// Copy video file with sanitized extension
		destPath := filepath.Join(assetsDir, fmt.Sprintf("video_%d%s", i, sanitizeFileExtension(video.Path)))
		size, err := ce.copyFile(video.Path, destPath)
		if err != nil {
			if ce.logger != nil {
				ce.logger.WarnCtx(ctx, "Failed to copy video", "path", video.Path, "error", err)
			}
			continue
		}

		// Update path in project
		project.Materials.Videos[i].Path = filepath.Join("assets", filepath.Base(destPath))

		assetCount++
		totalSize += size
	}

	// Package images
	for i, image := range project.Materials.Images {
		if image.Path == "" {
			continue
		}

		destPath := filepath.Join(assetsDir, fmt.Sprintf("image_%d%s", i, sanitizeFileExtension(image.Path)))
		size, err := ce.copyFile(image.Path, destPath)
		if err != nil {
			if ce.logger != nil {
				ce.logger.WarnCtx(ctx, "Failed to copy image", "path", image.Path, "error", err)
			}
			continue
		}

		project.Materials.Images[i].Path = filepath.Join("assets", filepath.Base(destPath))

		assetCount++
		totalSize += size
	}

	// Package audios
	for i, audio := range project.Materials.Audios {
		if audio.Path == "" {
			continue
		}

		destPath := filepath.Join(assetsDir, fmt.Sprintf("audio_%d%s", i, sanitizeFileExtension(audio.Path)))
		size, err := ce.copyFile(audio.Path, destPath)
		if err != nil {
			if ce.logger != nil {
				ce.logger.WarnCtx(ctx, "Failed to copy audio", "path", audio.Path, "error", err)
			}
			continue
		}

		project.Materials.Audios[i].Path = filepath.Join("assets", filepath.Base(destPath))

		assetCount++
		totalSize += size
	}

	// Rewrite draft.content with updated paths
	projectJSON, err := ExportCapCutProject(project)
	if err != nil {
		return assetCount, totalSize, fmt.Errorf("failed to serialize updated project: %w", err)
	}

	draftContentPath := filepath.Join(projectDir, "draft.content")
	if err := os.WriteFile(draftContentPath, projectJSON, 0644); err != nil {
		return assetCount, totalSize, fmt.Errorf("failed to write updated draft.content: %w", err)
	}

	return assetCount, totalSize, nil
}

// copyFile copies a file and returns its size
func (ce *CapCutExporter) copyFile(src, dst string) (int64, error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destFile.Close()

	size, err := io.Copy(destFile, sourceFile)
	if err != nil {
		// Clean up partially written file on error
		os.Remove(dst)
		return 0, err
	}

	return size, nil
}

// createZipPackage creates a zip file from a directory
func (ce *CapCutExporter) createZipPackage(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk the directory and add files to zip
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Create zip entry
		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipEntry, file)
		return err
	})
}

// ImportCapCutProject imports a CapCut project from JSON
func ImportCapCutProject(jsonData []byte) (*CapCutProject, error) {
	var project CapCutProject
	if err := json.Unmarshal(jsonData, &project); err != nil {
		return nil, fmt.Errorf("failed to parse CapCut project: %w", err)
	}

	return &project, nil
}

// ValidateCapCutProject performs basic validation on a CapCut project
func ValidateCapCutProject(project *CapCutProject) error {
	if project.Version == "" {
		return fmt.Errorf("project version is required")
	}

	if project.ID == "" {
		return fmt.Errorf("project ID is required")
	}

	if project.Duration <= 0 {
		return fmt.Errorf("project duration must be positive")
	}

	if project.CanvasConfig.Width <= 0 || project.CanvasConfig.Height <= 0 {
		return fmt.Errorf("invalid canvas dimensions: %dx%d", project.CanvasConfig.Width, project.CanvasConfig.Height)
	}

	if len(project.Tracks) == 0 {
		return fmt.Errorf("project must have at least one track")
	}

	return nil
}

// sanitizeFileExtension validates and returns a safe file extension
// Prevents path traversal attacks by only allowing safe extensions
func sanitizeFileExtension(path string) string {
	ext := filepath.Ext(path)

	// Only allow common media file extensions
	safeExtensions := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".mkv":  true,
		".webm": true,
		".mp3":  true,
		".wav":  true,
		".aac":  true,
		".m4a":  true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".bmp":  true,
		".webp": true,
	}

	// Convert to lowercase for comparison
	extLower := filepath.Ext(path)
	if len(extLower) > 0 {
		extLower = extLower[0:1] + filepath.Ext(path)[1:]
	}

	// Check if extension contains path separators
	if filepath.Base(ext) != ext {
		return ".mp4" // Default to safe extension
	}

	// Check if extension is in safe list
	if !safeExtensions[extLower] {
		return ".mp4" // Default to safe extension
	}

	return ext
}
