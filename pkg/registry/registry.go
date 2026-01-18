package registry

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/types"
)

var (
	mu       sync.RWMutex
	modules  = make(map[string]types.NexusModule)
	loadOnce sync.Once
)

func Register(name string, module types.NexusModule) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := modules[name]; exists {
		panic(fmt.Sprintf("module already registered: %s", name))
	}
	modules[name] = module
}

func LoadModules(cfg *config.Config) ([]types.NexusModule, error) {
	var loadErr error
	loadOnce.Do(func() {
		for name, module := range modules {
			if toggleable, ok := module.(interface {
				Enabled(cfg *config.Config) bool
			}); ok && !toggleable.Enabled(cfg) {
				slog.Info("module disabled", "name", name)
				continue
			}
			if err := module.Init(cfg); err != nil {
				loadErr = fmt.Errorf("failed to init module %s: %w", name, err)
				return
			}
		}
	})

	if loadErr != nil {
		return nil, loadErr
	}

	mu.RLock()
	defer mu.RUnlock()

	loaded := make([]types.NexusModule, 0, len(modules))
	for name, module := range modules {
		if toggleable, ok := module.(interface {
			Enabled(cfg *config.Config) bool
		}); ok && !toggleable.Enabled(cfg) {
			continue
		}
		slog.Info("module loaded", "name", name)
		loaded = append(loaded, module)
	}

	return loaded, nil
}
