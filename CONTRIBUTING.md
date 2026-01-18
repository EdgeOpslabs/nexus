# Contributing to Nexus

Thank you for helping build Nexus, a community-driven MCP server for EdgeOps Labs. This guide focuses on safe, modular contributions aligned with the plugin registry pattern.

## Development Basics

- Go version: 1.25+
- Config: `nexus.yaml` in repo root
- Logging: use `slog` and write to `os.Stderr` to avoid corrupting JSON-RPC stdout
- Error handling: wrap errors with context (e.g. `fmt.Errorf("failed to query prometheus: %w", err)`)
- Safe mode is the default: tools should be read-only unless explicitly designed otherwise

## Local Setup

```bash
go build -o nexus ./cmd/nexus
./nexus --config nexus.yaml
```

Run tests:

```bash
go test ./...
```

## Contribution Guidelines

- Keep tools token-efficient (support `tail_lines`, `since_seconds`, `contains`, `error_only` where applicable)
- Prefer structured summaries over raw dumps
- Enforce safe-mode defaults and policy checks
- Add read-only annotations to all non-mutating tools
- Keep secrets out of configs and manifests

## Security & Safe Mode

- Safe mode blocks destructive verbs by default
- If a tool can mutate state, require explicit confirmation or policy allowlist
- For log tools, enforce allowlisted paths and size limits

## Step-by-Step: Adding a New Tool

1. Create a module folder: `pkg/modules/<module-name>/`.
2. Implement the `NexusModule` interface from `pkg/types`.
3. Register the module in `init()` using `registry.Register`.
4. Add config fields in `pkg/config` and defaults in `nexus.yaml`.
5. Import the module package in `cmd/nexus/main.go` (blank import) so it self-registers.
6. Add `ReadOnlyHint`/`DestructiveHint` annotations.
7. Add tests for config defaults and module init paths.

## Adding a Plugin Bundle

Plugin bundles live under `modules.plugins.dir` (default `./plugins`), each with a `nexus.yaml` manifest:

```yaml
apiVersion: nexus/v1alpha1
kind: ContextServer
metadata:
  name: "example"
  vendor: "edgeops-labs"
  version: "v0.1.0"
  description: "Example plugin"
spec:
  command: "./bin/example-plugin"
  args: []
  env:
    LOG_LEVEL: "info"
  capabilities:
    tools:
      - name: "status"
        description: "Return a status string"
        read_only: true
        args:
          - name: "detail"
            type: "string"
            required: false
            description: "Extra detail"
```

Install locally:

```bash
./nexus install ./path/to/plugin-bundle --plugins-dir ./plugins
```

## Boilerplate Module Snippet

```go
package example

import (
  "context"
  "fmt"
  "log/slog"

  "github.com/edgeopslabs/nexus/pkg/config"
  "github.com/edgeopslabs/nexus/pkg/registry"
  "github.com/edgeopslabs/nexus/pkg/types"
  "github.com/mark3labs/mcp-go/mcp"
)

const (
  moduleName = "example"
  toolName   = "example_tool"
)

type Module struct {
  cfg *config.Config
}

func New() *Module {
  return &Module{}
}

func (m *Module) Name() string {
  return moduleName
}

func (m *Module) Init(cfg *config.Config) error {
  m.cfg = cfg
  slog.Info("example module initialized")
  return nil
}

func (m *Module) GetTools() []mcp.Tool {
  return []mcp.Tool{
    mcp.NewTool(toolName,
      mcp.WithDescription("Example tool."),
      mcp.WithString("input", mcp.Required(), mcp.Description("Example input")),
      mcp.WithReadOnlyHintAnnotation(true),
      mcp.WithDestructiveHintAnnotation(false),
    ),
  }
}

func (m *Module) HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
  switch name {
  case toolName:
    input, _ := args["input"].(string)
    if input == "" {
      return mcp.NewToolResultError("input is required"), nil
    }
    return mcp.NewToolResultText(fmt.Sprintf("received: %s", input)), nil
  default:
    return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
  }
}

func init() {
  registry.Register(moduleName, New())
}

var _ types.NexusModule = (*Module)(nil)
```

## PR Checklist

- [ ] Tests added or updated
- [ ] No stdout logging
- [ ] Safe mode behavior verified
- [ ] Tool annotations set (read-only/destructive)
- [ ] Docs updated (`GUIDE.md` or `README.md`) if needed
