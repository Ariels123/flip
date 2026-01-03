# GO EXPERT SKILL
## 1. Code Style
- Use standard Go formatting (`go fmt`).
- Prefer table-driven tests for unit testing.
- Handle all errors explicitly (`if err != nil`).
- Use `fmt.Errorf("context: %w", err)` for error wrapping.

## 2. Architecture
- **Supervisor**: Manages `WorkerPool`.
- **Worker**: Executes Tasks.
- **Coordinator**: High-level planning.
- **Avoid Global State**: Use struct dependencies.

## 3. Output Format
- Return **Active Code** only.
- No conversational filler ("Here is the code").
- Wrap code in ```go blocks.
