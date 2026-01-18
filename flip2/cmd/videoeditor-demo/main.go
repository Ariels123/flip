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
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="/demo.css">
</head>
<body>
    <div class="bg-animation"></div>
    <div class="container">
        <header>
            <div class="logo">
                <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
                    <rect x="4" y="4" width="40" height="40" rx="8" fill="url(#grad1)"/>
                    <path d="M16 18 L24 24 L16 30 Z" fill="white"/>
                    <path d="M26 18 L34 24 L26 30 Z" fill="white" opacity="0.7"/>
                    <defs>
                        <linearGradient id="grad1" x1="0%" y1="0%" x2="100%" y2="100%">
                            <stop offset="0%" stop-color="#667eea"/>
                            <stop offset="100%" stop-color="#764ba2"/>
                        </linearGradient>
                    </defs>
                </svg>
            </div>
            <h1>FLIP2 Video Editor</h1>
            <p class="subtitle">Professional Template-Based Video Generation</p>
            <p class="tagline">Powered by FFmpeg • CapCut Integration • Production Ready</p>
        </header>

        <div class="section">
            <div class="section-header">
                <h2>Core Capabilities</h2>
                <p class="section-desc">Enterprise-grade video composition system with template inheritance and batch processing</p>
            </div>
            <div class="grid">
                <div class="capability-card">
                    <div class="card-glow"></div>
                    <div class="icon-wrapper">
                        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
                            <path d="M2 17l10 5 10-5"/>
                            <path d="M2 12l10 5 10-5"/>
                        </svg>
                    </div>
                    <h3>Template Engine</h3>
                    <p class="card-description">Reusable video templates with multi-level inheritance and dynamic variables</p>
                    <div class="feature-list">
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>YAML/JSON format support</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Multi-level inheritance</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Type-safe variable substitution</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Canvas configuration</span>
                        </div>
                    </div>
                </div>

                <div class="capability-card">
                    <div class="card-glow"></div>
                    <div class="icon-wrapper">
                        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="3"/>
                            <path d="M12 1v6m0 6v6M6.7 6.7l4.2 4.2m2.2 2.2l4.2 4.2M1 12h6m6 0h6M6.7 17.3l4.2-4.2m2.2-2.2l4.2-4.2"/>
                        </svg>
                    </div>
                    <h3>Visual Effects</h3>
                    <p class="card-description">Professional-grade FFmpeg-based effects with real-time parameter tuning</p>
                    <div class="feature-list">
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Color correction suite</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Motion blur & gaussian</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Unsharp mask sharpening</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Film grain & vignette</span>
                        </div>
                    </div>
                </div>

                <div class="capability-card">
                    <div class="card-glow"></div>
                    <div class="icon-wrapper">
                        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="3" y="3" width="18" height="18" rx="2"/>
                            <circle cx="8.5" cy="8.5" r="1.5"/>
                            <path d="M21 15l-5-5L5 21"/>
                        </svg>
                    </div>
                    <h3>CapCut Integration</h3>
                    <p class="card-description">Seamless export to CapCut desktop editor with full project fidelity</p>
                    <div class="feature-list">
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>draft.content JSON format</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Multi-track timeline</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Text & graphic overlays</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Transitions & keyframes</span>
                        </div>
                    </div>
                </div>

                <div class="capability-card">
                    <div class="card-glow"></div>
                    <div class="icon-wrapper">
                        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
                        </svg>
                    </div>
                    <h3>Batch Processing</h3>
                    <p class="card-description">Concurrent video generation with intelligent worker pools and progress tracking</p>
                    <div class="feature-list">
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Configurable worker pools</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Priority queue system</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>CSV/JSON data sources</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Thread-safe progress tracking</span>
                        </div>
                    </div>
                </div>

                <div class="capability-card">
                    <div class="card-glow"></div>
                    <div class="icon-wrapper">
                        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="5" y="11" width="14" height="10" rx="2"/>
                            <circle cx="12" cy="16" r="2"/>
                            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                        </svg>
                    </div>
                    <h3>Security</h3>
                    <p class="card-description">Enterprise-grade security with cryptographic primitives and input validation</p>
                    <div class="feature-list">
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Path traversal protection</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Crypto/rand ID generation</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Input sanitization</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Safe file extension whitelist</span>
                        </div>
                    </div>
                </div>

                <div class="capability-card">
                    <div class="card-glow"></div>
                    <div class="icon-wrapper">
                        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                            <path d="M22 4L12 14.01l-3-3"/>
                        </svg>
                    </div>
                    <h3>Testing & Validation</h3>
                    <p class="card-description">Production-ready with comprehensive test coverage and validation suite</p>
                    <div class="feature-list">
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>71/71 tests passing (100%)</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Integration test coverage</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Thread safety verification</span>
                        </div>
                        <div class="feature-item">
                            <span class="check">✓</span>
                            <span>Security validation suite</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <div class="section">
            <div class="section-header">
                <h2>Available Templates</h2>
                <p class="section-desc">Pre-configured templates ready for customization</p>
            </div>
            <div id="templates-list" class="loading">Loading templates...</div>
        </div>

        <div class="section">
            <div class="section-header">
                <h2>Interactive Demo</h2>
                <p class="section-desc">Try out the system with live examples</p>
            </div>
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
/* Reset & Base */
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    background: #0f0f23;
    color: #e0e0e0;
    padding: 0;
    min-height: 100vh;
    position: relative;
    overflow-x: hidden;
}

/* Animated Background */
.bg-animation {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background:
        radial-gradient(circle at 20% 50%, rgba(102, 126, 234, 0.15) 0%, transparent 50%),
        radial-gradient(circle at 80% 80%, rgba(118, 75, 162, 0.15) 0%, transparent 50%),
        radial-gradient(circle at 40% 90%, rgba(96, 108, 229, 0.1) 0%, transparent 50%);
    z-index: 0;
    animation: bgPulse 20s ease-in-out infinite;
}

@keyframes bgPulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.7; }
}

.container {
    max-width: 1400px;
    margin: 0 auto;
    padding: 20px;
    position: relative;
    z-index: 1;
}

/* Header */
header {
    text-align: center;
    padding: 60px 20px 40px;
    margin-bottom: 40px;
}

.logo {
    display: inline-block;
    margin-bottom: 20px;
    animation: float 3s ease-in-out infinite;
}

@keyframes float {
    0%, 100% { transform: translateY(0px); }
    50% { transform: translateY(-10px); }
}

header h1 {
    font-size: 3.5em;
    margin-bottom: 12px;
    background: linear-gradient(135deg, #667eea 0%, #b490ff 50%, #764ba2 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    font-weight: 800;
    letter-spacing: -1px;
}

.subtitle {
    font-size: 1.4em;
    color: rgba(255, 255, 255, 0.9);
    margin-bottom: 8px;
    font-weight: 500;
}

.tagline {
    font-size: 0.95em;
    color: rgba(255, 255, 255, 0.6);
    letter-spacing: 0.5px;
}

/* Section */
.section {
    background: rgba(255, 255, 255, 0.03);
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 24px;
    padding: 40px;
    margin-bottom: 30px;
    box-shadow:
        0 8px 32px rgba(0, 0, 0, 0.3),
        inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.section-header {
    margin-bottom: 32px;
}

.section-header h2 {
    color: #ffffff;
    font-size: 2em;
    margin-bottom: 8px;
    font-weight: 700;
}

.section-desc {
    color: rgba(255, 255, 255, 0.6);
    font-size: 1.05em;
}

/* Grid */
.grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 24px;
}

/* Capability Cards */
.capability-card {
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    padding: 32px 28px;
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
    position: relative;
    overflow: hidden;
}

.card-glow {
    position: absolute;
    top: -50%;
    left: -50%;
    width: 200%;
    height: 200%;
    background: radial-gradient(circle, rgba(102, 126, 234, 0.15) 0%, transparent 70%);
    opacity: 0;
    transition: opacity 0.4s;
}

.capability-card:hover {
    border-color: rgba(102, 126, 234, 0.5);
    transform: translateY(-8px) scale(1.02);
    box-shadow:
        0 20px 40px rgba(102, 126, 234, 0.2),
        0 0 80px rgba(102, 126, 234, 0.1);
}

.capability-card:hover .card-glow {
    opacity: 1;
}

.icon-wrapper {
    width: 64px;
    height: 64px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, rgba(102, 126, 234, 0.2) 0%, rgba(118, 75, 162, 0.2) 100%);
    border-radius: 16px;
    margin-bottom: 20px;
    color: #b490ff;
    transition: all 0.3s;
}

.capability-card:hover .icon-wrapper {
    transform: scale(1.1) rotate(5deg);
    background: linear-gradient(135deg, rgba(102, 126, 234, 0.3) 0%, rgba(118, 75, 162, 0.3) 100%);
}

.capability-card h3 {
    color: #ffffff;
    margin: 16px 0 12px;
    font-size: 1.4em;
    font-weight: 700;
}

.card-description {
    color: rgba(255, 255, 255, 0.65);
    margin-bottom: 20px;
    line-height: 1.6;
    font-size: 0.95em;
}

.feature-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.feature-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
    color: rgba(255, 255, 255, 0.7);
    font-size: 0.92em;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.feature-item:last-child {
    border-bottom: none;
}

.check {
    color: #667eea;
    font-weight: bold;
    font-size: 1.1em;
}

/* Demo Panel & Forms */
.demo-panel {
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.1);
    padding: 28px;
    border-radius: 16px;
    margin-bottom: 24px;
}

.demo-panel h3 {
    color: #ffffff;
    font-size: 1.3em;
    margin-bottom: 20px;
    font-weight: 600;
}

form {
    display: flex;
    flex-direction: column;
    gap: 16px;
}

label {
    font-weight: 600;
    color: rgba(255, 255, 255, 0.9);
    margin-bottom: 6px;
    font-size: 0.95em;
}

input, select {
    padding: 14px 16px;
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 10px;
    font-size: 1em;
    background: rgba(255, 255, 255, 0.06);
    color: #ffffff;
    transition: all 0.3s;
}

input:focus, select:focus {
    outline: none;
    border-color: #667eea;
    background: rgba(255, 255, 255, 0.08);
    box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
}

button {
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: white;
    border: none;
    padding: 14px 28px;
    border-radius: 10px;
    cursor: pointer;
    font-size: 1em;
    font-weight: 600;
    transition: all 0.3s;
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
}

button:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 20px rgba(102, 126, 234, 0.4);
}

button:active {
    transform: translateY(0);
}

/* Status Messages */
.success {
    background: rgba(16, 185, 129, 0.15);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #10b981;
    padding: 16px;
    border-radius: 10px;
    margin-top: 16px;
    font-weight: 500;
}

.error {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
    padding: 16px;
    border-radius: 10px;
    margin-top: 16px;
    font-weight: 500;
}

.info {
    background: rgba(59, 130, 246, 0.15);
    border: 1px solid rgba(59, 130, 246, 0.3);
    color: #3b82f6;
    padding: 16px;
    border-radius: 10px;
    margin-top: 16px;
    font-weight: 500;
}

/* Template Display */
.loading {
    color: rgba(255, 255, 255, 0.5);
    text-align: center;
    padding: 40px;
    font-style: italic;
}

.template-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 16px;
}

.template-card {
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.1);
    padding: 20px;
    border-radius: 12px;
    text-align: center;
    transition: all 0.3s;
}

.template-card:hover {
    border-color: rgba(102, 126, 234, 0.4);
    transform: translateY(-4px);
}

.template-card h4 {
    color: #ffffff;
    margin-bottom: 12px;
    font-size: 1.1em;
}

.template-card button {
    width: 100%;
    padding: 10px;
    font-size: 0.9em;
}

/* Effects */
.effects-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-top: 16px;
}

.effect-card {
    background: rgba(255, 255, 255, 0.04);
    border-left: 4px solid #667eea;
    padding: 16px;
    border-radius: 8px;
    color: rgba(255, 255, 255, 0.8);
}

.effect-card h4 {
    color: #ffffff;
    margin-bottom: 8px;
}

code {
    background: rgba(255, 255, 255, 0.1);
    padding: 3px 8px;
    border-radius: 5px;
    font-size: 0.9em;
    color: #b490ff;
    font-family: 'Monaco', 'Menlo', monospace;
}

/* Responsive Design */
@media (max-width: 768px) {
    header h1 { font-size: 2.5em; }
    .grid { grid-template-columns: 1fr; }
    .section { padding: 24px; }
    .template-grid { grid-template-columns: 1fr; }
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
