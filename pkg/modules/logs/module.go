package logs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/registry"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	moduleName  = "logs"
	tailLogTool = "logs_tail"
	grepLogTool = "logs_grep"
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
	return cfg.Modules.Logs.Enabled
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	if !cfg.Modules.Logs.Enabled {
		slog.Info("logs module disabled by config")
	}
	return nil
}

func (m *Module) GetTools() []mcp.Tool {
	if m.cfg == nil || !m.cfg.Modules.Logs.Enabled {
		return nil
	}

	return []mcp.Tool{
		mcp.NewTool(tailLogTool,
			mcp.WithDescription("Tail logs from a local file path (safe-mode: allowlisted paths only)."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Log file path.")),
			mcp.WithNumber("tail_lines", mcp.Description("Number of lines to return (default 200).")),
			mcp.WithString("contains", mcp.Description("Filter lines containing this substring (case-insensitive).")),
			mcp.WithBoolean("error_only", mcp.Description("Only include common error patterns (recommended).")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTool(grepLogTool,
			mcp.WithDescription("Search logs from a local file path (safe-mode: allowlisted paths only)."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Log file path.")),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search substring.")),
			mcp.WithNumber("max_lines", mcp.Description("Max matching lines to return (default 200).")),
			mcp.WithBoolean("case_sensitive", mcp.Description("Case-sensitive search (default false).")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
	}
}

func (m *Module) HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if !m.cfg.Modules.Logs.Enabled {
		return mcp.NewToolResultError("logs module is disabled"), nil
	}

	switch name {
	case tailLogTool:
		return m.handleTail(ctx, args)
	case grepLogTool:
		return m.handleGrep(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
	}
}

func (m *Module) handleTail(_ context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	path := getStringArg(args, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}
	absPath, err := m.validatePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tailLines := clampInt(getIntArg(args, "tail_lines", 200), 1, m.cfg.Modules.Logs.MaxLines)
	contains := strings.TrimSpace(getStringArg(args, "contains", ""))
	errorOnly := getBoolArg(args, "error_only", true)

	content, err := readTail(absPath, tailLines, m.cfg.Modules.Logs.MaxBytes)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read logs: %v", err)), nil
	}

	content = filterLines(content, contains, false, tailLines, errorOnly)

	if content == "" {
		content = "(no matching log lines)"
	}
	return mcp.NewToolResultText(content), nil
}

func (m *Module) handleGrep(_ context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	path := getStringArg(args, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}
	query := getStringArg(args, "query", "")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	absPath, err := m.validatePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	maxLines := clampInt(getIntArg(args, "max_lines", 200), 1, m.cfg.Modules.Logs.MaxLines)
	caseSensitive := getBoolArg(args, "case_sensitive", false)

	content, err := readTail(absPath, maxLines*10, m.cfg.Modules.Logs.MaxBytes)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read logs: %v", err)), nil
	}

	content = filterLines(content, query, caseSensitive, maxLines, false)
	if content == "" {
		content = "(no matching log lines)"
	}
	return mcp.NewToolResultText(content), nil
}

func (m *Module) validatePath(input string) (string, error) {
	if len(m.cfg.Modules.Logs.AllowPaths) == 0 {
		return "", fmt.Errorf("log access denied: allow_paths is empty")
	}

	abs, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	for _, root := range m.cfg.Modules.Logs.AllowPaths {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rootResolved, err := filepath.EvalSymlinks(rootAbs)
		if err != nil {
			continue
		}
		if strings.HasPrefix(resolved, rootResolved+string(os.PathSeparator)) || resolved == rootResolved {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("log path not allowed: %s", resolved)
}

func readTail(path string, tailLines int, maxBytes int) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	size := info.Size()
	limit := int64(maxBytes)
	start := int64(0)
	if size > limit {
		start = size - limit
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return strings.Join(lines, "\n"), nil
}

func filterLines(content, query string, caseSensitive bool, maxLines int, errorOnly bool) string {
	needle := query
	if !caseSensitive {
		needle = strings.ToLower(query)
	}
	lines := strings.Split(content, "\n")
	var filtered []string
	for _, line := range lines {
		hay := line
		if !caseSensitive {
			hay = strings.ToLower(line)
		}
		if query != "" && !strings.Contains(hay, needle) {
			continue
		}
		if errorOnly && !matchesErrorPattern(hay) {
			continue
		}
		filtered = append(filtered, line)
		if len(filtered) >= maxLines {
			break
		}
	}
	return strings.Join(filtered, "\n")
}

func matchesErrorPattern(line string) bool {
	patterns := []string{
		"error",
		"failed",
		"panic",
		"fatal",
		"exception",
		"crash",
		"backoff",
		"oom",
		"terminated",
		"refused",
		"timeout",
	}
	for _, pattern := range patterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
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
				var n int
				_, err := fmt.Sscanf(v, "%d", &n)
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
