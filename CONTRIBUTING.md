# Contributing to Nexus

Thank you for helping build Nexus, a community-driven MCP server for EdgeOps Labs. This guide focuses on adding new tools and modules with the registry pattern.

## Development Basics

- Go version: 1.25+
- Config: `nexus.yaml` in repo root
- Logging: use `slog` and write to `os.Stderr` to avoid corrupting JSON-RPC stdout
- Error handling: wrap errors with context (e.g. `fmt.Errorf("failed to query prometheus: %w", err)`)

## Step-by-Step: Adding a New Tool

1. Create a module folder: `pkg/modules/<module-name>/`.
2. Implement the `NexusModule` interface from `pkg/types`.
3. Register the module in `init()` using `registry.Register`.
4. Add config fields in `pkg/config` and defaults in `nexus.yaml`.
5. Import the module package in `cmd/nexus/main.go` (blank import) so it self-registers.

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
