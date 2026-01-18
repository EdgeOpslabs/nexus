package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/edgeopslabs/nexus/pkg/config"
	pluginapi "github.com/edgeopslabs/nexus/pkg/plugins"
	"github.com/edgeopslabs/nexus/pkg/registry"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

const moduleName = "plugins"

type Module struct {
	cfg       *config.Config
	manifests []pluginapi.Manifest
	tools     map[string]pluginTool
}

type pluginTool struct {
	manifest pluginapi.Manifest
	tool     pluginapi.ToolSpec
}

func New() *Module {
	return &Module{
		tools: make(map[string]pluginTool),
	}
}

func (m *Module) Name() string {
	return moduleName
}

func (m *Module) Enabled(cfg *config.Config) bool {
	return cfg.Modules.Plugins.Enabled
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	if !cfg.Modules.Plugins.Enabled {
		slog.Info("plugins module disabled by config")
		return nil
	}

	manifests, err := pluginapi.LoadManifests(cfg.Modules.Plugins.Dir)
	if err != nil {
		slog.Warn("failed to load plugin manifests", "error", err)
		return nil
	}
	m.manifests = manifests

	for _, manifest := range manifests {
		for _, tool := range manifest.Spec.Capabilities.Tools {
			fullName := pluginToolName(manifest.Metadata.Name, tool.Name)
			if _, exists := m.tools[fullName]; exists {
				return fmt.Errorf("duplicate plugin tool name: %s", fullName)
			}
			m.tools[fullName] = pluginTool{manifest: manifest, tool: tool}
		}
	}
	return nil
}

func (m *Module) GetTools() []mcp.Tool {
	if m.cfg == nil || !m.cfg.Modules.Plugins.Enabled {
		return nil
	}
	var tools []mcp.Tool
	for name, pt := range m.tools {
		tool := buildToolSchema(name, pt.tool)
		tools = append(tools, tool)
	}
	return tools
}

func (m *Module) HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if !m.cfg.Modules.Plugins.Enabled {
		return mcp.NewToolResultError("plugins module is disabled"), nil
	}

	pt, ok := m.tools[name]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
	}

	if m.cfg.Server.SafeMode && !pt.tool.ReadOnly {
		return mcp.NewToolResultError("tool blocked in safe mode"), nil
	}

	payload := map[string]any{
		"tool": pt.tool.Name,
		"args": args,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin input: %w", err)
	}

	cmdPath := pt.manifest.Spec.Command
	if cmdPath == "" {
		return mcp.NewToolResultError("plugin command not configured"), nil
	}
	if !filepath.IsAbs(cmdPath) {
		cmdPath = filepath.Join(m.cfg.Modules.Plugins.Dir, pt.manifest.Metadata.Name, cmdPath)
	}

	cmdArgs := append([]string{}, pt.manifest.Spec.Args...)
	cmdArgs = append(cmdArgs, "--nexus-plugin")
	cmd := exec.CommandContext(ctx, cmdPath, cmdArgs...)
	cmd.Env = append(cmd.Env, m.cfg.Modules.Plugins.Env...)
	for key, value := range pt.manifest.Spec.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Stdin = strings.NewReader(string(data))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("plugin error: %v: %s", err, strings.TrimSpace(string(output)))), nil
	}
	return mcp.NewToolResultText(trimOutput(string(output), m.cfg.Modules.Plugins.MaxBytes)), nil
}

func buildToolSchema(name string, spec pluginapi.ToolSpec) mcp.Tool {
	tool := mcp.NewTool(name,
		mcp.WithDescription(spec.Description),
	)
	properties := make(map[string]any)
	var required []string
	for _, arg := range spec.Args {
		properties[arg.Name] = map[string]any{
			"type":        normalizeType(arg.Type),
			"description": arg.Description,
		}
		if arg.Required {
			required = append(required, arg.Name)
		}
	}
	tool.InputSchema = mcp.ToolInputSchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
	tool.Annotations.ReadOnlyHint = &spec.ReadOnly
	destructive := !spec.ReadOnly
	tool.Annotations.DestructiveHint = &destructive
	return tool
}

func normalizeType(value string) string {
	switch strings.ToLower(value) {
	case "int", "integer":
		return "integer"
	case "bool", "boolean":
		return "boolean"
	case "number", "float":
		return "number"
	default:
		return "string"
	}
}

func pluginToolName(pluginName, toolName string) string {
	return fmt.Sprintf("plugin/%s/%s", pluginName, toolName)
}

func trimOutput(output string, maxBytes int) string {
	if maxBytes <= 0 {
		return output
	}
	data := []byte(output)
	if len(data) <= maxBytes {
		return output
	}
	return string(data[:maxBytes]) + "\n... truncated"
}

func init() {
	registry.Register(moduleName, New())
}

var _ types.NexusModule = (*Module)(nil)
