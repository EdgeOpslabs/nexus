package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/registry"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	moduleName       = "docker"
	listContainers   = "docker_list_containers"
	inspectContainer = "docker_inspect_container"
	containerLogs    = "docker_get_logs"
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

func (m *Module) Enabled(cfg *config.Config) bool {
	return cfg.Modules.Docker.Enabled
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	if !cfg.Modules.Docker.Enabled {
		slog.Info("docker module disabled by config")
	}
	return nil
}

func (m *Module) GetTools() []mcp.Tool {
	if m.cfg == nil || !m.cfg.Modules.Docker.Enabled {
		return nil
	}

	return []mcp.Tool{
		mcp.NewTool(listContainers,
			mcp.WithDescription("List running Docker containers."),
			mcp.WithBoolean("all", mcp.Description("Include stopped containers.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTool(inspectContainer,
			mcp.WithDescription("Inspect a Docker container."),
			mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTool(containerLogs,
			mcp.WithDescription("Fetch Docker container logs."),
			mcp.WithString("id", mcp.Required(), mcp.Description("Container ID or name.")),
			mcp.WithNumber("tail_lines", mcp.Description("Max log lines to return (default 200).")),
			mcp.WithNumber("since_seconds", mcp.Description("Only return logs newer than this many seconds.")),
			mcp.WithBoolean("timestamps", mcp.Description("Include timestamps in log output.")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
	}
}

func (m *Module) HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if !m.cfg.Modules.Docker.Enabled {
		return mcp.NewToolResultError("docker module is disabled"), nil
	}

	switch name {
	case listContainers:
		return m.handleList(ctx, args)
	case inspectContainer:
		return m.handleInspect(ctx, args)
	case containerLogs:
		return m.handleLogs(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
	}
}

func (m *Module) handleList(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	all := getBoolArg(args, "all", false)
	argv := []string{"ps", "--format", "{{.ID}} {{.Image}} {{.Status}} {{.Names}}"}
	if all {
		argv = append(argv, "-a")
	}
	output, err := m.runDocker(ctx, argv...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("docker ps failed: %v", err)), nil
	}
	if strings.TrimSpace(output) == "" {
		output = "(no containers found)"
	}
	return mcp.NewToolResultText(output), nil
}

func (m *Module) handleInspect(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	id := getStringArg(args, "id", "")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}
	output, err := m.runDocker(ctx, "inspect", id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("docker inspect failed: %v", err)), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (m *Module) handleLogs(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	id := getStringArg(args, "id", "")
	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}
	tailLines := clampInt(getIntArg(args, "tail_lines", 200), 1, m.cfg.Modules.Docker.MaxLines)
	sinceSeconds := getIntArg(args, "since_seconds", 0)
	timestamps := getBoolArg(args, "timestamps", false)

	argv := []string{"logs", "--tail", strconv.Itoa(tailLines)}
	if sinceSeconds > 0 {
		argv = append(argv, "--since", strconv.Itoa(sinceSeconds))
	}
	if timestamps {
		argv = append(argv, "--timestamps")
	}
	argv = append(argv, id)

	output, err := m.runDocker(ctx, argv...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("docker logs failed: %v", err)), nil
	}
	if strings.TrimSpace(output) == "" {
		output = "(no log lines)"
	}
	return mcp.NewToolResultText(output), nil
}

func (m *Module) runDocker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, m.cfg.Modules.Docker.CLI, args...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(data)))
	}
	return strings.TrimSpace(string(data)), nil
}

func getStringArg(args map[string]interface{}, key, def string) string {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return def
}

func getIntArg(args map[string]interface{}, key string, def int) int {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if v != "" {
				n, err := strconv.Atoi(v)
				if err == nil {
					return n
				}
			}
		}
	}
	return def
}

func getBoolArg(args map[string]interface{}, key string, def bool) bool {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			switch strings.ToLower(v) {
			case "true", "1", "yes", "y":
				return true
			case "false", "0", "no", "n":
				return false
			}
		}
	}
	return def
}

func clampInt(value, min, max int) int {
	if max <= 0 {
		max = 200
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func init() {
	registry.Register(moduleName, New())
}

var _ types.NexusModule = (*Module)(nil)
