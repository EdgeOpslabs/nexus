package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/registry"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	moduleName      = "prometheus"
	queryMetricTool = "prometheus_query_metric"
)

type Module struct {
	cfg    *config.Config
	client *http.Client
}

func New() *Module {
	return &Module{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *Module) Name() string {
	return moduleName
}

func (m *Module) Enabled(cfg *config.Config) bool {
	return cfg.Modules.Prometheus.Enabled
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	if !cfg.Modules.Prometheus.Enabled {
		slog.Info("prometheus module disabled by config")
	}
	return nil
}

func (m *Module) GetTools() []mcp.Tool {
	if m.cfg == nil || !m.cfg.Modules.Prometheus.Enabled {
		return nil
	}

	return []mcp.Tool{
		mcp.NewTool(queryMetricTool,
			mcp.WithDescription("Query a Prometheus metric using PromQL."),
			mcp.WithString("query", mcp.Required(), mcp.Description("PromQL query string")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
	}
}

func (m *Module) HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if !m.cfg.Modules.Prometheus.Enabled {
		return mcp.NewToolResultError("prometheus module is disabled"), nil
	}

	switch name {
	case queryMetricTool:
		return m.handleQueryMetric(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
	}
}

func (m *Module) handleQueryMetric(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	endpoint, err := buildQueryURL(m.cfg.Modules.Prometheus.URL, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid prometheus url: %v", err)), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build prometheus request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return mcp.NewToolResultError(fmt.Sprintf("prometheus returned status %d", resp.StatusCode)), nil
	}

	var payload prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode prometheus response: %w", err)
	}
	if payload.Status != "success" {
		if payload.Error != "" {
			return mcp.NewToolResultError(fmt.Sprintf("prometheus error: %s", payload.Error)), nil
		}
		return mcp.NewToolResultError("prometheus query failed"), nil
	}

	result := formatResult(payload, query)
	return mcp.NewToolResultText(result), nil
}

type prometheusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   struct {
		ResultType string                   `json:"resultType"`
		Result     []prometheusVectorResult `json:"result"`
	} `json:"data"`
}

type prometheusVectorResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

func buildQueryURL(baseURL, query string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "", fmt.Errorf("base url is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base url must include scheme and host")
	}

	path := strings.TrimRight(parsed.Path, "/")
	parsed.Path = path + "/api/v1/query"

	q := parsed.Query()
	q.Set("query", query)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

func formatResult(payload prometheusResponse, query string) string {
	if len(payload.Data.Result) == 0 {
		return fmt.Sprintf("prometheus: query=%q returned no data", query)
	}

	lines := make([]string, 0, len(payload.Data.Result)+1)
	lines = append(lines, fmt.Sprintf("prometheus: query=%q resultType=%s", query, payload.Data.ResultType))
	for _, item := range payload.Data.Result {
		value := formatValue(item.Value)
		lines = append(lines, fmt.Sprintf("- metric=%s value=%s", formatMetric(item.Metric), value))
	}
	return strings.Join(lines, "\n")
}

func formatMetric(metric map[string]string) string {
	if len(metric) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(metric))
	for key, value := range metric {
		parts = append(parts, fmt.Sprintf("%s=%q", key, value))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatValue(value []interface{}) string {
	if len(value) < 2 {
		return "unknown"
	}
	return fmt.Sprintf("%v @ %v", value[1], value[0])
}

func init() {
	registry.Register(moduleName, New())
}

var _ types.NexusModule = (*Module)(nil)
