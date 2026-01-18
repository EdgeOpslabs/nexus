package policy

import (
	"testing"

	"github.com/edgeopslabs/nexus/pkg/config"
)

func TestPolicyDenyOverrides(t *testing.T) {
	cfg := config.PolicyConfig{
		DenyTools: []string{"k8s_list_pods"},
	}
	p := New(cfg, false)
	if p.Evaluate("kubernetes", "k8s_list_pods") != Deny {
		t.Fatalf("expected deny")
	}
}

func TestPolicyAllowList(t *testing.T) {
	cfg := config.PolicyConfig{
		AllowTools: []string{"prometheus_query_metric"},
	}
	p := New(cfg, false)
	if p.Evaluate("kubernetes", "k8s_list_pods") != Deny {
		t.Fatalf("expected deny when allowlist does not match")
	}
	if p.Evaluate("prometheus", "prometheus_query_metric") != Allow {
		t.Fatalf("expected allow for allowlisted tool")
	}
}

func TestPolicyConfirm(t *testing.T) {
	cfg := config.PolicyConfig{
		ConfirmTools: []string{"kubernetes/k8s_list_pods"},
	}
	p := New(cfg, false)
	if p.Evaluate("kubernetes", "k8s_list_pods") != Confirm {
		t.Fatalf("expected confirm")
	}
}

func TestSafeModeBlocksSensitive(t *testing.T) {
	p := New(config.PolicyConfig{}, true)
	if p.Evaluate("kubernetes", "k8s_delete_pod") != Deny {
		t.Fatalf("expected deny for sensitive tool in safe mode")
	}
}
