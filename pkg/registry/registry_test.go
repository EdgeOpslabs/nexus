package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

type testModule struct {
	enabled  bool
	initRuns int
}

func (t *testModule) Name() string { return "test" }
func (t *testModule) Enabled(cfg *config.Config) bool {
	return t.enabled
}
func (t *testModule) Init(cfg *config.Config) error {
	t.initRuns++
	return nil
}
func (t *testModule) GetTools() []mcp.Tool { return nil }
func (t *testModule) HandleCall(_ context.Context, _ string, _ map[string]interface{}) (*mcp.CallToolResult, error) {
	return nil, nil
}

func resetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	modules = make(map[string]types.NexusModule)
	loadOnce = sync.Once{}
}

func TestLoadModulesSkipsDisabled(t *testing.T) {
	resetRegistry()
	module := &testModule{enabled: false}
	Register("test", module)

	cfg := config.DefaultConfig()
	loaded, err := LoadModules(cfg)
	if err != nil {
		t.Fatalf("load modules: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected no loaded modules")
	}
	if module.initRuns != 0 {
		t.Fatalf("expected Init not to run for disabled module")
	}
}

func TestLoadModulesInitRunsOnce(t *testing.T) {
	resetRegistry()
	module := &testModule{enabled: true}
	Register("test", module)

	cfg := config.DefaultConfig()
	if _, err := LoadModules(cfg); err != nil {
		t.Fatalf("load modules: %v", err)
	}
	if _, err := LoadModules(cfg); err != nil {
		t.Fatalf("load modules second time: %v", err)
	}
	if module.initRuns != 1 {
		t.Fatalf("expected Init to run once, got %d", module.initRuns)
	}
}
