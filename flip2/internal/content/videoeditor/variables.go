// Package videoeditor - Variable substitution for templates.
package videoeditor

import (
	"fmt"
	"regexp"
	"strings"
)

// VariableSubstitutor handles variable replacement in templates.
type VariableSubstitutor struct {
	variables map[string]interface{}
	pattern   *regexp.Regexp
}

// NewVariableSubstitutor creates a new variable substitutor.
func NewVariableSubstitutor(variables map[string]interface{}) *VariableSubstitutor {
	return &VariableSubstitutor{
		variables: variables,
		// Matches {{variable}} or {{variable|default}}
		pattern: regexp.MustCompile(`\{\{([^}|]+)(?:\|([^}]+))?\}\}`),
	}
}

// Substitute replaces all {{variable}} placeholders in the input string.
// Supports default values with {{variable|default}} syntax.
// Supports nested variables with {{user.name}} syntax.
func (vs *VariableSubstitutor) Substitute(input string) (string, error) {
	result := vs.pattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name and optional default value
		parts := vs.pattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		varName := strings.TrimSpace(parts[1])
		defaultValue := ""
		if len(parts) >= 3 {
			defaultValue = parts[2]
		}

		// Get variable value
		value, err := vs.getValue(varName)
		if err != nil {
			// Use default value if variable not found
			if defaultValue != "" {
				return defaultValue
			}
			// Return original placeholder if no default
			return match
		}

		// Convert value to string
		return fmt.Sprintf("%v", value)
	})

	return result, nil
}

// SubstituteTemplate replaces variables in a complete template.
func (vs *VariableSubstitutor) SubstituteTemplate(template *Template) (*Template, error) {
	// Create a deep copy
	result := &Template{
		Name:      template.Name,
		Version:   template.Version,
		Canvas:    template.Canvas,
		Timeline:  make([]TimelineItem, len(template.Timeline)),
		Effects:   template.Effects,
		Variables: template.Variables,
		Metadata:  template.Metadata,
	}

	// Substitute variables in timeline items
	for i, item := range template.Timeline {
		result.Timeline[i] = item

		// Substitute path
		if item.Path != "" {
			path, err := vs.Substitute(item.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to substitute path in timeline item %d: %w", i, err)
			}
			result.Timeline[i].Path = path
		}

		// Substitute content
		if item.Content != "" {
			content, err := vs.Substitute(item.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to substitute content in timeline item %d: %w", i, err)
			}
			result.Timeline[i].Content = content
		}

		// Substitute text style properties
		if item.Style != nil {
			if item.Style.FontFile != "" {
				fontFile, err := vs.Substitute(item.Style.FontFile)
				if err != nil {
					return nil, fmt.Errorf("failed to substitute font file in timeline item %d: %w", i, err)
				}
				result.Timeline[i].Style.FontFile = fontFile
			}

			if item.Style.FontColor != "" {
				fontColor, err := vs.Substitute(item.Style.FontColor)
				if err != nil {
					return nil, fmt.Errorf("failed to substitute font color in timeline item %d: %w", i, err)
				}
				result.Timeline[i].Style.FontColor = fontColor
			}
		}
	}

	return result, nil
}

// getValue retrieves a variable value, supporting nested paths (e.g., "user.name").
func (vs *VariableSubstitutor) getValue(path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := vs.variables

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - return the value
			if val, ok := current[part]; ok {
				return val, nil
			}
			return nil, fmt.Errorf("variable not found: %s", path)
		}

		// Navigate deeper
		if val, ok := current[part]; ok {
			// Check if value is a map for further navigation
			if m, ok := val.(map[string]interface{}); ok {
				current = m
			} else {
				return nil, fmt.Errorf("cannot navigate path %s: %s is not a map", path, part)
			}
		} else {
			return nil, fmt.Errorf("variable not found: %s", path)
		}
	}

	return nil, fmt.Errorf("variable not found: %s", path)
}

// SetVariable sets a variable value, supporting nested paths.
func (vs *VariableSubstitutor) SetVariable(path string, value interface{}) error {
	parts := strings.Split(path, ".")

	if len(parts) == 1 {
		// Simple variable
		vs.variables[path] = value
		return nil
	}

	// Nested variable - ensure parent maps exist
	current := vs.variables
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		if val, ok := current[part]; ok {
			// Navigate to existing map
			if m, ok := val.(map[string]interface{}); ok {
				current = m
			} else {
				return fmt.Errorf("cannot set %s: %s is not a map", path, part)
			}
		} else {
			// Create intermediate map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
	return nil
}

// GetVariable retrieves a variable value.
func (vs *VariableSubstitutor) GetVariable(path string) (interface{}, error) {
	return vs.getValue(path)
}

// HasVariable checks if a variable exists.
func (vs *VariableSubstitutor) HasVariable(path string) bool {
	_, err := vs.getValue(path)
	return err == nil
}

// MergeVariables merges additional variables into the substitutor.
// Existing variables are overwritten by new values.
func (vs *VariableSubstitutor) MergeVariables(newVars map[string]interface{}) {
	for k, v := range newVars {
		vs.variables[k] = v
	}
}
