package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/edgeopslabs/nexus/pkg/common"
	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/plugins"
	"github.com/edgeopslabs/nexus/pkg/policy"
	"github.com/edgeopslabs/nexus/pkg/registry"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	_ "github.com/edgeopslabs/nexus/pkg/modules/docker"
	_ "github.com/edgeopslabs/nexus/pkg/modules/kubernetes"
	_ "github.com/edgeopslabs/nexus/pkg/modules/logs"
	_ "github.com/edgeopslabs/nexus/pkg/modules/plugins"
	_ "github.com/edgeopslabs/nexus/pkg/modules/prometheus"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install" {
		runInstall(os.Args[2:])
		return
	}

	common.PrintBanner()

	configPath := flag.String("config", "nexus.yaml", "path to nexus configuration file")
	safeMode := flag.Bool("safe-mode", false, "run in read-only safe mode")
	transport := flag.String("transport", "stdio", "transport: stdio or sse")
	httpAddr := flag.String("http-addr", ":8080", "http listen address for sse transport")
	baseURL := flag.String("base-url", "", "base URL for sse endpoint (e.g. http://localhost:8080)")
	basePath := flag.String("base-path", "/mcp", "base path for sse endpoints")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if *safeMode {
		cfg.Server.SafeMode = true
	}
	configureLogging(cfg)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("config file not found, using defaults", "path", *configPath)
		} else {
			slog.Warn("failed to load config, using defaults", "path", *configPath, "error", err)
		}
	}
	if cfg.Server.SafeMode {
		slog.Warn("safe mode enabled (read-only)")
	}

	// 1. Initialize the Nexus Server
	s := server.NewMCPServer(
		cfg.Server.Name,
		cfg.Server.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	modules, err := registry.LoadModules(cfg)
	if err != nil {
		slog.Error("failed to load modules", "error", err)
		os.Exit(1)
	}

	toolPolicy := policy.New(cfg.Policy, cfg.Server.SafeMode)
	toolSummaries := collectToolSummaries(modules, toolPolicy)
	registerTools(s, modules, toolPolicy)

	if strings.ToLower(*transport) == "sse" {
		startSSEServer(s, cfg.Server.Name, cfg.Server.Version, toolSummaries, *httpAddr, *baseURL, *basePath)
		return
	}

	// 3. Start the Server (Stdio Transport)
	// AI Agents (Claude/Cursor) talk to this binary via Stdin/Stdout
	fmt.Fprintln(os.Stderr, "🔌 Nexus is connecting to the matrix...")

	if err := server.ServeStdio(s); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func configureLogging(cfg *config.Config) {
	level := parseLogLevel(cfg.Server.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func registerTools(s *server.MCPServer, modules []types.NexusModule, toolPolicy *policy.Policy) {
	for _, module := range modules {
		mod := module
		for _, tool := range mod.GetTools() {
			toolName := tool.Name
			name := toolName
			decision := toolPolicy.Evaluate(mod.Name(), toolName)
			if decision == policy.Deny {
				slog.Warn("tool blocked by policy", "module", mod.Name(), "tool", toolName)
				continue
			}

			s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				callDecision := toolPolicy.Evaluate(mod.Name(), name)
				if callDecision == policy.Deny {
					return mcp.NewToolResultError("tool blocked by policy"), nil
				}
				if callDecision == policy.Confirm {
					if !confirmTool(mod.Name(), name) {
						return mcp.NewToolResultError("tool execution denied by user"), nil
					}
				}
				args, ok := request.Params.Arguments.(map[string]interface{})
				if !ok {
					args = make(map[string]interface{})
				}
				return mod.HandleCall(ctx, name, args)
			})
			slog.Info("tool registered", "module", mod.Name(), "tool", toolName)
		}
	}
}

type toolSummary struct {
	Module      string `json:"module"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
}

type toolInventory struct {
	Server    string        `json:"server"`
	Version   string        `json:"version"`
	Transport string        `json:"transport"`
	Tools     []toolSummary `json:"tools"`
}

func collectToolSummaries(modules []types.NexusModule, toolPolicy *policy.Policy) []toolSummary {
	var summaries []toolSummary
	for _, module := range modules {
		for _, tool := range module.GetTools() {
			decision := toolPolicy.Evaluate(module.Name(), tool.Name)
			status := "allowed"
			if decision == policy.Confirm {
				status = "confirm"
			} else if decision == policy.Deny {
				status = "denied"
			}
			summaries = append(summaries, toolSummary{
				Module:      module.Name(),
				Name:        tool.Name,
				Description: tool.Description,
				Status:      status,
			})
		}
	}
	return summaries
}

func startSSEServer(mcpServer *server.MCPServer, name, version string, tools []toolSummary, addr, baseURL, basePath string) {
	if baseURL == "" {
		baseURL = "http://localhost" + addr
	}

	sseServer := server.NewSSEServer(
		mcpServer,
		server.WithBaseURL(baseURL),
		server.WithStaticBasePath(basePath),
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
		server.WithUseFullURLForMessageEndpoint(true),
		server.WithKeepAlive(true),
	)

	mux := http.NewServeMux()
	mux.Handle(basePath+"/sse", sseServer.SSEHandler())
	mux.Handle(basePath+"/message", sseServer.MessageHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/tools", func(w http.ResponseWriter, _ *http.Request) {
		payload := toolInventory{
			Server:    name,
			Version:   version,
			Transport: "sse",
			Tools:     tools,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})

	slog.Info("starting sse server", "addr", addr, "baseURL", baseURL, "basePath", basePath)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("sse server error", "error", err)
		os.Exit(1)
	}
}

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	pluginsDir := fs.String("plugins-dir", "plugins", "plugins directory")
	_ = fs.Parse(args)
	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nexus install <path-or-url> [--plugins-dir plugins]")
		os.Exit(2)
	}
	source := remaining[0]

	installedPath, err := plugins.Install(source, *pluginsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Installed plugin bundle at %s\n", installedPath)
}

func confirmTool(module, tool string) bool {
	tty, err := os.OpenFile(filepath.Clean("/dev/tty"), os.O_RDWR, 0)
	if err != nil {
		slog.Warn("confirmation unavailable; denying tool", "module", module, "tool", tool, "error", err)
		return false
	}
	defer tty.Close()

	_, _ = fmt.Fprintf(tty, "Confirm execution of %s/%s [y/N]: ", module, tool)
	reader := bufio.NewReader(tty)
	line, _ := reader.ReadString('\n')
	response := strings.TrimSpace(strings.ToLower(line))
	return response == "y" || response == "yes"
}
