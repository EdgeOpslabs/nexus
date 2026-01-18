package types

import (
	"context"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/mark3labs/mcp-go/mcp"
)

type NexusModule interface {
	Name() string
	Init(cfg *config.Config) error
	GetTools() []mcp.Tool
	HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error)
}
