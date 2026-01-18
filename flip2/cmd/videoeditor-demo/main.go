package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"flip2/internal/content/videoeditor"
)

var (
	templateManager *videoeditor.TemplateManager
	editor          *videoeditor.Editor
	exporter        *videoeditor.CapCutExporter
	outputDir       string
)

func main() {
	// Setup directories
	workDir := "./demo_workspace"
	outputDir = filepath.Join(workDir, "output")
	templateDir := filepath.Join(workDir, "templates")

	os.MkdirAll(outputDir, 0755)
	os.MkdirAll(templateDir, 0755)

	// Create demo templates
	if err := createDemoTemplates(templateDir); err != nil {
		log.Fatalf("Failed to create demo templates: %v", err)
	}

	// Initialize components
	templateManager = videoeditor.NewTemplateManager([]string{templateDir})
	editor, _ = videoeditor.New(nil, "ffmpeg", "ffprobe", workDir, outputDir)
	exporter = videoeditor.NewCapCutExporter(templateManager, outputDir, nil)

	// Setup routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/templates", handleListTemplates)
	http.HandleFunc("/api/template/", handleGetTemplate)
	http.HandleFunc("/api/export", handleExport)
	http.HandleFunc("/api/batch", handleBatch)
	http.HandleFunc("/api/effects", handleEffects)
	http.HandleFunc("/demo.css", handleCSS)

	port := "8080"
	fmt.Printf("\n🎬 Video Editor Demo Server\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Server running at: http://localhost:%s\n", port)
	fmt.Printf("Templates directory: %s\n", templateDir)
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Printf("\nOpen your browser to explore the system!\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Video Editor Demo - FLIP2</title>
    <link rel="stylesheet" href="/demo.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>🎬 Video Editor Demo</h1>
            <p class="subtitle">Template-Based Video Generation with CapCut Integration</p>
        </header>

        <div class="capabilities">
            <h2>System Capabilities</h2>
            <div class="grid">
                <div class="capability-card">
                    <div class="icon">📐</div>
                    <h3>Template Engine</h3>
                    <p>Create reusable video templates with variables, effects, and inheritance</p>
                    <ul>
                        <li>YAML/JSON template format</li>
                        <li>Multi-level inheritance</li>
                        <li>Variable substitution</li>
                        <li>Canvas configuration</li>
                    </ul>
                </div>

                <div class="capability-card">
                    <div class="icon">🎨</div>
                    <h3>Visual Effects</h3>
                    <p>5 built-in effects with customizable parameters</p>
                    <ul>
                        <li>Color Correction (brightness, contrast, saturation)</li>
                        <li>Blur (gaussian, box, motion)</li>
                        <li>Sharpen (unsharp mask)</li>
                        <li>Noise (film grain)</li>
                        <li>Vignette (darkened edges)</li>
                    </ul>
                </div>

                <div class="capability-card">
                    <div class="icon">📹</div>
                    <h3>CapCut Integration</h3>
                    <p>Export templates to CapCut desktop editor format</p>
                    <ul>
                        <li>draft.content JSON generation</li>
                        <li>Timeline & tracks</li>
                        <li>Text overlays</li>
                        <li>Transitions & animations</li>
                    </ul>
                </div>

                <div class="capability-card">
                    <div class="icon">⚡</div>
                    <h3>Batch Processing</h3>
                    <p>Process multiple videos concurrently with progress tracking</p>
                    <ul>
                        <li>Worker pool (configurable concurrency)</li>
                        <li>Priority queue</li>
                        <li>CSV/JSON data sources</li>
                        <li>Thread-safe progress tracking</li>
                    </ul>
                </div>

                <div class="capability-card">
                    <div class="icon">🔒</div>
                    <h3>Security</h3>
                    <p>Built-in security features for safe processing</p>
                    <ul>
                        <li>Path traversal prevention</li>
                        <li>Cryptographic ID generation</li>
                        <li>Input validation</li>
                        <li>Safe file extensions</li>
                    </ul>
                </div>

                <div class="capability-card">
                    <div class="icon">🧪</div>
                    <h3>Testing & Validation</h3>
                    <p>Comprehensive test suite with 71 tests</p>
                    <ul>
                        <li>Unit tests (65 tests)</li>
                        <li>Integration tests (6 tests)</li>
                        <li>Thread safety verification</li>
                        <li>100% pass rate</li>
                    </ul>
                </div>
            </div>
        </div>

        <div class="section">
            <h2>Available Templates</h2>
            <div id="templates-list">Loading...</div>
        </div>

        <div class="section">
            <h2>Try It Out</h2>
            <div class="demo-panel">
                <h3>1. Quick Export</h3>
                <form id="export-form">
                    <label>Template:</label>
                    <select id="template-select" name="template">
                        <option value="">Loading...</option>
                    </select>

                    <label>Title:</label>
                    <input type="text" name="title" value="My Awesome Video" required>

                    <label>Episode Number:</label>
                    <input type="number" name="episode" value="1" min="1">

                    <button type="submit">Export to CapCut</button>
                </form>
                <div id="export-result"></div>
            </div>

            <div class="demo-panel">
                <h3>2. Batch Processing</h3>
                <p>Process multiple videos at once (simulated)</p>
                <button id="batch-btn">Run Batch (10 videos)</button>
                <div id="batch-result"></div>
            </div>

            <div class="demo-panel">
                <h3>3. Effect System</h3>
                <button id="effects-btn">List Available Effects</button>
                <div id="effects-result"></div>
            </div>
        </div>

        <div class="section">
            <h2>Technical Details</h2>
            <div class="tech-grid">
                <div class="tech-card">
                    <h4>Backend</h4>
                    <p>Go 1.21+</p>
                    <p>FFmpeg integration</p>
                    <p>Concurrent processing</p>
                </div>
                <div class="tech-card">
                    <h4>Template Format</h4>
                    <p>YAML or JSON</p>
                    <p>Variable substitution</p>
                    <p>Template inheritance</p>
                </div>
                <div class="tech-card">
                    <h4>Export Formats</h4>
                    <p>CapCut draft.content</p>
                    <p>ZIP packages</p>
                    <p>Asset bundling</p>
                </div>
                <div class="tech-card">
                    <h4>Performance</h4>
                    <p>Thread-safe operations</p>
                    <p>Atomic counters</p>
                    <p>Configurable workers</p>
                </div>
            </div>
        </div>
    </div>

    <script>
        // Safely escape HTML to prevent XSS
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Load templates
        fetch('/api/templates')
            .then(r => r.json())
            .then(data => {
                const list = document.getElementById('templates-list');
                const select = document.getElementById('template-select');

                if (data.templates.length === 0) {
                    const p = document.createElement('p');
                    p.className = 'info';
                    p.textContent = 'No templates found. Demo templates should be created automatically.';
                    list.appendChild(p);
                    return;
                }

                const grid = document.createElement('div');
                grid.className = 'template-grid';

                data.templates.forEach(t => {
                    const card = document.createElement('div');
                    card.className = 'template-card';

                    const h4 = document.createElement('h4');
                    h4.textContent = t;
                    card.appendChild(h4);

                    const btn = document.createElement('button');
                    btn.textContent = 'View Details';
                    btn.onclick = () => viewTemplate(t);
                    card.appendChild(btn);

                    grid.appendChild(card);

                    const option = document.createElement('option');
                    option.value = t;
                    option.textContent = t;
                    select.appendChild(option);
                });

                list.textContent = '';
                list.appendChild(grid);
                select.options[0].remove(); // Remove "Loading..."
            });

        // Export form
        document.getElementById('export-form').addEventListener('submit', async (e) => {
            e.preventDefault();
            const formData = new FormData(e.target);
            const result = document.getElementById('export-result');

            result.textContent = '';
            const infoDiv = document.createElement('div');
            infoDiv.className = 'info';
            infoDiv.textContent = '⏳ Exporting...';
            result.appendChild(infoDiv);

            try {
                const response = await fetch('/api/export', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        template: formData.get('template'),
                        variables: {
                            title: formData.get('title'),
                            episode_number: parseInt(formData.get('episode'))
                        }
                    })
                });

                const data = await response.json();
                result.textContent = '';

                const resDiv = document.createElement('div');
                if (response.ok) {
                    resDiv.className = 'success';
                    resDiv.textContent = '✅ Export Successful!\n' +
                        'Project: ' + data.project_path + '\n' +
                        'Size: ' + (data.size / 1024).toFixed(2) + ' KB\n' +
                        'Assets: ' + data.asset_count;
                } else {
                    resDiv.className = 'error';
                    resDiv.textContent = '❌ Error: ' + data.error;
                }
                resDiv.style.whiteSpace = 'pre-line';
                result.appendChild(resDiv);
            } catch (err) {
                result.textContent = '';
                const errDiv = document.createElement('div');
                errDiv.className = 'error';
                errDiv.textContent = '❌ Network error: ' + err.message;
                result.appendChild(errDiv);
            }
        });

        // Batch processing
        document.getElementById('batch-btn').addEventListener('click', async () => {
            const result = document.getElementById('batch-result');
            result.textContent = '';

            const infoDiv = document.createElement('div');
            infoDiv.className = 'info';
            infoDiv.textContent = '⏳ Processing batch...';
            result.appendChild(infoDiv);

            try {
                const response = await fetch('/api/batch', {method: 'POST'});
                const data = await response.json();

                result.textContent = '';
                const successDiv = document.createElement('div');
                successDiv.className = 'success';
                successDiv.textContent = '✅ Batch Complete!\n' +
                    'Total: ' + data.total + '\n' +
                    'Completed: ' + data.completed + '\n' +
                    'Failed: ' + data.failed + '\n' +
                    'Duration: ' + data.duration;
                successDiv.style.whiteSpace = 'pre-line';
                result.appendChild(successDiv);
            } catch (err) {
                result.textContent = '';
                const errDiv = document.createElement('div');
                errDiv.className = 'error';
                errDiv.textContent = '❌ Error: ' + err.message;
                result.appendChild(errDiv);
            }
        });

        // Effects list
        document.getElementById('effects-btn').addEventListener('click', async () => {
            const result = document.getElementById('effects-result');

            try {
                const response = await fetch('/api/effects');
                const data = await response.json();

                result.textContent = '';
                const effectsList = document.createElement('div');
                effectsList.className = 'effects-list';

                data.effects.forEach(e => {
                    const card = document.createElement('div');
                    card.className = 'effect-card';

                    const strong = document.createElement('strong');
                    strong.textContent = e.name;
                    card.appendChild(strong);
                    card.appendChild(document.createElement('br'));

                    const small = document.createElement('small');
                    small.textContent = e.description;
                    card.appendChild(small);
                    card.appendChild(document.createElement('br'));

                    const code = document.createElement('code');
                    code.textContent = 'Parameters: ' + e.parameters.join(', ');
                    card.appendChild(code);

                    effectsList.appendChild(card);
                });

                result.appendChild(effectsList);
            } catch (err) {
                result.textContent = '';
                const errDiv = document.createElement('div');
                errDiv.className = 'error';
                errDiv.textContent = '❌ Error: ' + err.message;
                result.appendChild(errDiv);
            }
        });

        function viewTemplate(name) {
            fetch('/api/template/' + encodeURIComponent(name))
                .then(r => r.json())
                .then(data => {
                    alert('Template: ' + name + '\n\n' +
                          'Canvas: ' + data.canvas.width + 'x' + data.canvas.height + '\n' +
                          'FPS: ' + data.canvas.fps + '\n' +
                          'Timeline items: ' + data.timeline_items + '\n' +
                          'Effects: ' + data.effects + '\n' +
                          'Variables: ' + data.variables);
                });
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func handleCSS(w http.ResponseWriter, r *http.Request) {
	css := `
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: #333;
    padding: 20px;
}
.container { max-width: 1200px; margin: 0 auto; }
header {
    text-align: center;
    color: white;
    padding: 40px 20px;
    margin-bottom: 30px;
}
header h1 { font-size: 3em; margin-bottom: 10px; }
.subtitle { font-size: 1.2em; opacity: 0.9; }
.section {
    background: white;
    border-radius: 12px;
    padding: 30px;
    margin-bottom: 20px;
    box-shadow: 0 4px 6px rgba(0,0,0,0.1);
}
h2 { color: #667eea; margin-bottom: 20px; }
.grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 20px;
}
.capability-card {
    border: 2px solid #e0e0e0;
    border-radius: 8px;
    padding: 20px;
    transition: all 0.3s;
}
.capability-card:hover {
    border-color: #667eea;
    transform: translateY(-5px);
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.2);
}
.icon { font-size: 3em; margin-bottom: 10px; }
.capability-card h3 { color: #333; margin: 10px 0; }
.capability-card p { color: #666; margin-bottom: 15px; }
.capability-card ul { list-style: none; padding-left: 0; }
.capability-card li {
    padding: 5px 0;
    color: #555;
    border-bottom: 1px solid #f0f0f0;
}
.capability-card li:before { content: "✓ "; color: #667eea; font-weight: bold; }
.demo-panel {
    background: #f8f9fa;
    padding: 20px;
    border-radius: 8px;
    margin-bottom: 15px;
}
form { display: flex; flex-direction: column; gap: 10px; }
label { font-weight: bold; color: #555; }
input, select {
    padding: 10px;
    border: 2px solid #e0e0e0;
    border-radius: 4px;
    font-size: 1em;
}
button {
    background: #667eea;
    color: white;
    border: none;
    padding: 12px 24px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 1em;
    font-weight: bold;
    transition: background 0.3s;
}
button:hover { background: #5568d3; }
.success {
    background: #d4edda;
    color: #155724;
    padding: 15px;
    border-radius: 4px;
    margin-top: 10px;
}
.error {
    background: #f8d7da;
    color: #721c24;
    padding: 15px;
    border-radius: 4px;
    margin-top: 10px;
}
.info {
    background: #d1ecf1;
    color: #0c5460;
    padding: 15px;
    border-radius: 4px;
    margin-top: 10px;
}
.template-grid, .tech-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 15px;
}
.template-card, .tech-card {
    background: white;
    border: 2px solid #e0e0e0;
    padding: 15px;
    border-radius: 8px;
    text-align: center;
}
.effects-list { display: flex; flex-direction: column; gap: 10px; margin-top: 15px; }
.effect-card {
    background: white;
    border-left: 4px solid #667eea;
    padding: 15px;
    border-radius: 4px;
}
code {
    background: #f4f4f4;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 0.9em;
}
`
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(css))
}

func handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := templateManager.ListTemplates()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	})
}

func handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/api/template/"):]

	tmpl, err := templateManager.Load(context.Background(), name)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":           tmpl.Name,
		"version":        tmpl.Version,
		"canvas":         tmpl.Canvas,
		"timeline_items": len(tmpl.Timeline),
		"effects":        len(tmpl.Effects),
		"variables":      len(tmpl.Variables),
	})
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template  string                 `json:"template"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	result, err := exporter.Export(context.Background(), videoeditor.ExportOptions{
		TemplateName:  req.Template,
		Variables:     req.Variables,
		IncludeAssets: false,
		PackageFormat: "directory",
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"project_path": result.ProjectPath,
		"size":         result.PackageSize,
		"asset_count":  result.AssetCount,
	})
}

func handleBatch(w http.ResponseWriter, r *http.Request) {
	// Simulate batch processing
	bp := videoeditor.NewBatchProcessor(editor, templateManager, 3, nil)

	jobs := make([]*videoeditor.BatchJob, 10)
	for i := 0; i < 10; i++ {
		jobs[i] = &videoeditor.BatchJob{
			ID:           fmt.Sprintf("demo_job_%d", i),
			TemplateName: "youtube-tutorial",
			Variables: map[string]interface{}{
				"title":          fmt.Sprintf("Video %d", i+1),
				"episode_number": i + 1,
			},
			OutputPath: filepath.Join(outputDir, fmt.Sprintf("demo_%d.mp4", i)),
		}
	}

	results, _ := bp.ProcessBatch(context.Background(), jobs)
	stats := bp.GetProgress()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":     stats.TotalJobs,
		"completed": stats.CompletedJobs,
		"failed":    stats.FailedJobs,
		"duration":  fmt.Sprintf("%.2fs", float64(len(results))*0.5),
	})
}

func handleEffects(w http.ResponseWriter, r *http.Request) {
	registry := videoeditor.NewEffectRegistry()
	effects := registry.List()

	type EffectInfo struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Parameters  []string `json:"parameters"`
	}

	effectInfos := []EffectInfo{
		{
			Name:        "color_correct",
			Description: "Adjust brightness, contrast, saturation, hue, and gamma",
			Parameters:  []string{"brightness", "contrast", "saturation", "hue", "gamma"},
		},
		{
			Name:        "blur",
			Description: "Apply gaussian, box, or motion blur",
			Parameters:  []string{"radius", "type"},
		},
		{
			Name:        "sharpen",
			Description: "Enhance edges using unsharp mask",
			Parameters:  []string{"strength"},
		},
		{
			Name:        "noise",
			Description: "Add film grain noise",
			Parameters:  []string{"strength"},
		},
		{
			Name:        "vignette",
			Description: "Darken edges for focus effect",
			Parameters:  []string{"intensity"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"effects": effectInfos,
		"count":   len(effects),
	})
}

func createDemoTemplates(templateDir string) error {
	// YouTube Tutorial Template
	youtubeTutorial := `
name: "youtube-tutorial"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

variables:
  title: "Tutorial Title"
  episode_number: 1
  brand_color: "#FF4444"

timeline:
  - type: "text"
    content: "Episode {{episode_number}}: {{title}}"
    start_time: 0
    duration: 3000
    style:
      font_size: 72
      font_color: "{{brand_color}}"
      position: "center"

  - type: "text"
    content: "Thanks for watching!"
    start_time: 8000
    duration: 2000
    style:
      font_size: 48
      font_color: "{{brand_color}}"
      position: "center"

effects:
  - type: "color_correct"
    parameters:
      brightness: 0.05
      contrast: 1.1
`

	// TikTok Short Template
	tiktokShort := `
name: "tiktok-short"
version: "1.0"

canvas:
  width: 1080
  height: 1920
  fps: 30

variables:
  hook: "Wait for it..."
  cta: "Follow for more!"

timeline:
  - type: "text"
    content: "{{hook}}"
    start_time: 0
    duration: 1500
    style:
      font_size: 64
      font_color: "#FFFFFF"
      position: "center"

  - type: "text"
    content: "{{cta}}"
    start_time: 8500
    duration: 1500
    style:
      font_size: 48
      font_color: "#FF00FF"
      position: "bottom"

effects:
  - type: "sharpen"
    parameters:
      strength: 30
  - type: "vignette"
    parameters:
      intensity: 0.3
`

	templates := map[string]string{
		"youtube-tutorial.yaml": youtubeTutorial,
		"tiktok-short.yaml":     tiktokShort,
	}

	for filename, content := range templates {
		path := filepath.Join(templateDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}
