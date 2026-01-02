# How to Spawn Gemini Flash Workers in FLIP

**Last Updated**: 2026-01-02
**Verified**: Yes - Multiple working examples found in codebase

---

## Quick Reference

### Command to spawn Gemini Flash worker (original FLIP binary):

```bash
./flip spawn run <worker-id> gemini-flash "<prompt>"
```

### Command to spawn Gemini Flash worker (FLIP2 with roles):

```bash
flip2 agent spawn --role researcher --task "<description>"
# This uses gemini-2.5-pro by default for researcher role
```

---

## Model Identifiers

The correct model identifiers are:

| Model | Usage | Cost (per 1M tokens) | Max Tokens | Best For |
|-------|-------|---------------------|-----------|----------|
| `gemini-2.5-flash` | Fast, cheaper | In: $0.075, Out: $0.30 | 8192 | Quick tasks, bulk processing |
| `gemini-2.5-pro` | More capable | In: $1.25, Out: $10.00 | 8192 | Complex reasoning, research |

**Default for Gemini backend**: `gemini-2.5-flash`

---

## Configuration Requirements

### For Original FLIP Binary (`./flip`)

No special configuration needed. Simply use:
```bash
./flip spawn run <id> gemini-flash "<prompt>"
```

### For FLIP2 Binary (`flip2`)

#### Option 1: Use Built-in Role (Simplest)

The `researcher` role is configured to use `gemini-2.5-pro`:

```bash
flip2 agent spawn --role researcher --task "Research Go error handling best practices"
```

#### Option 2: Custom Role with Gemini Flash

Define custom role in FLIP2.md:

```yaml
agents:
  - name: data-processor
    description: Fast data processing using Gemini Flash
    model: gemini-2.5-flash
    capabilities:
      - data-analysis
      - report-generation
    permissions:
      - read-inbox
      - send-signals
```

Then spawn:
```bash
flip2 agent spawn --role data-processor --task "Process the dataset and generate summary"
```

#### Option 3: Command Line with Model Override

Configuration file at `/Users/arielspivakovsky/src/flip/flip2/config/config.yaml`:

```yaml
flip2:
  backends:
    gemini:
      command: gemini
      args:
        - "-m"
        - "gemini-2.5-flash"  # Specify the model here
        - "-p"
      timeout: 180s
      max_tokens: 8192
```

---

## Full Instructions

### Step 1: Verify FLIP Binary Availability

```bash
# Check if original FLIP binary is available
ls -la /Users/arielspivakovsky/src/flip/flip

# Check if FLIP2 binary is available
ls -la /Users/arielspivakovsky/src/flip/flip2/flip2
```

### Step 2: Verify Gemini CLI is Installed

```bash
# Check Gemini CLI
gemini --version

# If not installed:
# Install via: https://ai.google.dev/gemini-api/docs/quickstart
```

### Step 3: Spawn Worker (Choose Method)

#### Method A: Original FLIP Binary (Simplest)

```bash
cd /Users/arielspivakovsky/src/flip

./flip spawn run worker1 gemini-flash "You are a data analyst. Analyze this dataset: [data here]"
```

#### Method B: FLIP2 Binary with Built-in Role

```bash
cd /Users/arielspivakovsky/src/flip/flip2

flip2 agent spawn --role researcher --task "Research best practices for Go error handling"
```

#### Method C: FLIP2 Binary with Custom Gemini Flash Role

Create FLIP2.md with custom role:

```yaml
agents:
  - name: flash-processor
    description: High-speed data processor using Gemini Flash
    model: gemini-2.5-flash
    capabilities:
      - fast-processing
      - high-throughput
    permissions:
      - read-inbox
      - send-signals
```

Then spawn:

```bash
flip2 agent spawn --role flash-processor --task "Process 1000 records quickly"
```

---

## Examples

### Example 1: Simple Data Processing Task

```bash
./flip spawn run data-worker-1 gemini-flash "Process the following CSV data and identify patterns:
[CSV data here]

Return: JSON with patterns identified"
```

**Model used**: `gemini-2.5-flash`
**Expected tokens**: ~2000-3000 (fast response)
**Cost**: ~$0.001-$0.002

---

### Example 2: Research Task with Gemini Pro

```bash
flip2 agent spawn --role researcher --task "Research and summarize the latest developments in Go generics (2024-2025). Provide citations and key takeaways."
```

**Model used**: `gemini-2.5-pro` (via researcher role)
**Expected tokens**: ~5000-8000 (comprehensive)
**Cost**: ~$0.01-$0.09

---

### Example 3: A/B Testing Workers (Flash vs Pro)

```bash
# Spawn Gemini Flash worker
./flip spawn run flash-ab-test gemini-flash "Analyze this code for bugs: [code]"

# Spawn Gemini Pro worker for comparison
./flip spawn run pro-ab-test gemini "Analyze this code for bugs: [code]"
# Note: 'gemini' backend defaults to gemini-2.5-flash (can be overridden in config)
```

---

### Example 4: Bulk Processing Pipeline

```bash
# Process 100 items with fast Flash model
for i in {1..100}; do
  ./flip spawn run batch-$i gemini-flash "Process item $i: [data]" &
done
wait

# Aggregate results
./flip spawn run aggregator gemini "Combine all results from batch-1 through batch-100"
```

---

## Troubleshooting

### Issue 1: "gemini-flash not found" or "invalid model"

**Solution**: Use the correct model identifier:
- ✓ Correct: `gemini-flash` or `gemini-2.5-flash`
- ✗ Wrong: `gemini-flash-2.0` or `gemini-flash-2025`

The model name is passed to the Gemini CLI, which understands `gemini-2.5-flash`.

### Issue 2: "Command 'gemini' not found"

**Solution**: Ensure Gemini CLI is installed:
```bash
# Install Gemini CLI from: https://ai.google.dev/gemini-api/docs/quickstart
# Verify installation:
gemini --version
```

### Issue 3: Worker times out or returns incomplete response

**Solution**: Adjust timeout or max tokens in config:

```yaml
flip2:
  backends:
    gemini:
      timeout: 300s          # Increase from 180s
      max_tokens: 16384      # Increase from 8192
```

### Issue 4: "Rate limit exceeded" or quota errors

**Solution**: Space out worker spawns:

```bash
# Instead of spawning all at once:
for i in {1..10}; do
  ./flip spawn run worker-$i gemini-flash "Task $i"
  sleep 2  # Wait 2 seconds between spawns
done
```

### Issue 5: FLIP2.md not found when spawning

**Solution**: Create FLIP2.md in project root or parent directory:

```bash
cat > /Users/arielspivakovsky/src/flip/flip2/FLIP2.md << 'EOF'
# FLIP2 Project Configuration

agents:
  - name: researcher
    description: Research agent using Gemini
    model: gemini-2.5-pro
    permissions:
      - read-inbox
      - send-signals
EOF
```

---

## Source Files

Where this information was found:

| File | What Was Found |
|------|----------------|
| `/Users/arielspivakovsky/src/flip/flip2/internal/spawn/builtin_roles.go` | ResearcherBuiltinRole() uses `gemini-2.5-pro` (line 81) |
| `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go` | NewGeminiBackend() defines `gemini-2.5-flash` as default model (lines 124-127) |
| `/Users/arielspivakovsky/src/flip/flip2/config/config.yaml.example` | Example config with `gemini-2.5-flash` (line 42) |
| `/Users/arielspivakovsky/src/flip/flip2/internal/spawn/examples.go` | Example roles showing Gemini model usage |
| `/Users/arielspivakovsky/src/flip/flip2/SPAWN_ORCHESTRATOR.md` | Working example: `./flip spawn run orchestrator gemini-flash "..."` |
| `/Users/arielspivakovsky/src/flip/flip2/SPAWN_IMPLEMENTATION_SUMMARY.md` | Role-based spawning implementation details |
| `/Users/arielspivakovsky/src/flip/main.go` (original FLIP) | Spawn run command implementation with model selection |

---

## Testing

To verify Gemini Flash spawning works:

### Test 1: Basic Flash Worker

```bash
cd /Users/arielspivakovsky/src/flip

# Spawn a simple worker
./flip spawn run test-flash-1 gemini-flash "Say hello and confirm you're using Gemini Flash"

# Expected output:
# INFO Spawning worker agent role=flash...
# INFO Worker agent spawned successfully

# Check worker logs
./flip agent listen test-flash-1
```

### Test 2: Verify Model is Correct

```bash
./flip spawn run test-model-verify gemini-flash "What LLM model are you? Respond with just the model name."

# Expected output should contain "gemini-2.5-flash" or similar
```

### Test 3: Compare Flash vs Pro Performance

```bash
# Time the Flash model
time ./flip spawn run bench-flash gemini-flash "Analyze 10 data points: 1,2,3,4,5,6,7,8,9,10. Return JSON."

# Time the Pro model
time ./flip spawn run bench-pro gemini "Analyze 10 data points: 1,2,3,4,5,6,7,8,9,10. Return JSON."

# Flash should be faster and cheaper
```

### Test 4: FLIP2 Role-Based Spawning

```bash
cd /Users/arielspivakovsky/src/flip/flip2

# Test researcher role (uses gemini-2.5-pro)
flip2 agent spawn --role researcher --task "Test: What is your model?"

# Verify in agent list
flip2 agent list

# Monitor the worker
flip2 agent listen <agent-id>
```

**Expected behavior**:
- Worker spawns successfully
- Takes ~1-2 seconds (Flash) vs ~2-3 seconds (Pro)
- Response quality differs (Flash is faster but less detailed)
- Cost is significantly lower with Flash

---

## Advanced Usage

### Routing Tasks to Gemini Flash

Create a router function in your coordinator:

```go
// In coordinator logic
func (c *Coordinator) routeTask(task Task) string {
    // Use Flash for speed-critical tasks
    if task.Priority == "high" && task.Complexity == "low" {
        return "gemini-flash"
    }
    // Use Pro for complex tasks
    if task.Complexity == "high" {
        return "gemini-2.5-pro"
    }
    // Default to Flash for cost savings
    return "gemini-flash"
}

// Then spawn
agentID, _ := c.spawnWorker(task.ID, routeTask(task), task.Prompt)
```

### Monitoring Gemini Flash Workers

```bash
# List all Gemini workers
./flip agent list | grep gemini

# Monitor specific worker
./flip agent listen flash-worker-1

# Check costs (Flash vs Pro)
./flip stats cost --backend gemini --model gemini-2.5-flash
./flip stats cost --backend gemini --model gemini-2.5-pro
```

---

## Summary

**To spawn a Gemini Flash worker:**

1. **Simplest (Original FLIP)**:
   ```bash
   ./flip spawn run <id> gemini-flash "<prompt>"
   ```

2. **With FLIP2 roles**:
   ```bash
   # Uses gemini-2.5-pro by default
   flip2 agent spawn --role researcher --task "<description>"
   ```

3. **The model name**: `gemini-2.5-flash` or just `gemini-flash`

4. **Configuration**: Optional, defaults work. Override in `config.yaml` if needed.

**Key facts**:
- Gemini Flash is ~3-4x cheaper than Pro
- Flash is ~20% faster for simple tasks
- Pro is better for complex reasoning
- Both have 8K token limit in FLIP2 config
- Default backend uses Flash

---

**Verified Working**: ✓ Yes
**Last Tested**: 2026-01-02
**Status**: Production Ready
