package policy

import (
	"path"
	"strings"

	"github.com/edgeopslabs/nexus/pkg/config"
)

type Decision int

const (
	Allow Decision = iota
	Deny
	Confirm
)

type Policy struct {
	cfg      config.PolicyConfig
	safeMode bool
}

func New(cfg config.PolicyConfig, safeMode bool) *Policy {
	return &Policy{cfg: cfg, safeMode: safeMode}
}

func (p *Policy) Evaluate(module, tool string) Decision {
	if p.safeMode && isSensitiveTool(tool) {
		return Deny
	}

	if matchesAny(p.cfg.DenyModules, module) || matchesAnyTool(p.cfg.DenyTools, module, tool) {
		return Deny
	}

	if hasAllowList(p.cfg) && !matchesAny(p.cfg.AllowModules, module) && !matchesAnyTool(p.cfg.AllowTools, module, tool) {
		return Deny
	}

	if matchesAnyTool(p.cfg.ConfirmTools, module, tool) {
		return Confirm
	}

	return Allow
}

func hasAllowList(cfg config.PolicyConfig) bool {
	return len(cfg.AllowModules) > 0 || len(cfg.AllowTools) > 0
}

func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, value); matched {
			return true
		}
	}
	return false
}

func matchesAnyTool(patterns []string, module, tool string) bool {
	qualified := module + "/" + tool
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, tool); matched {
			return true
		}
		if matched, _ := path.Match(pattern, qualified); matched {
			return true
		}
	}
	return false
}

func isSensitiveTool(tool string) bool {
	lower := strings.ToLower(tool)
	sensitive := []string{"delete", "update", "scale", "write", "create", "apply", "patch"}
	for _, keyword := range sensitive {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
