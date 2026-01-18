package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type NexusConfig struct {
	Server  ServerConfig  `yaml:"server"`
	Modules ModulesConfig `yaml:"modules"`
	Policy  PolicyConfig  `yaml:"policy"`
}

type Config = NexusConfig

type ServerConfig struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	LogLevel string `yaml:"log_level"`
	SafeMode bool   `yaml:"safe_mode"`
}

type PolicyConfig struct {
	AllowModules []string `yaml:"allow_modules"`
	DenyModules  []string `yaml:"deny_modules"`
	AllowTools   []string `yaml:"allow_tools"`
	DenyTools    []string `yaml:"deny_tools"`
	ConfirmTools []string `yaml:"confirm_tools"`
}

type ModulesConfig struct {
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	AWS        AWSConfig        `yaml:"aws"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Logs       LogsConfig       `yaml:"logs"`
	Docker     DockerConfig     `yaml:"docker"`
	Plugins    PluginsConfig    `yaml:"plugins"`
}

type KubernetesConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Kubeconfig string `yaml:"kubeconfig"`
}

type AWSConfig struct {
	Enabled bool   `yaml:"enabled"`
	Region  string `yaml:"region"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

type LogsConfig struct {
	Enabled    bool     `yaml:"enabled"`
	AllowPaths []string `yaml:"allow_paths"`
	MaxBytes   int      `yaml:"max_bytes"`
	MaxLines   int      `yaml:"max_lines"`
}

type DockerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CLI      string `yaml:"cli"`
	MaxLines int    `yaml:"max_lines"`
}

type PluginsConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Dir      string   `yaml:"dir"`
	MaxBytes int      `yaml:"max_bytes"`
	Env      []string `yaml:"env"`
}

func DefaultConfig() *NexusConfig {
	return &NexusConfig{
		Server: ServerConfig{
			Name:     "Nexus",
			Version:  "v0.0.1",
			LogLevel: "info",
			SafeMode: true,
		},
		Modules: ModulesConfig{
			Kubernetes: KubernetesConfig{
				Enabled:    true,
				Kubeconfig: "~/.kube/config",
			},
			AWS: AWSConfig{
				Enabled: false,
				Region:  "us-east-1",
			},
			Prometheus: PrometheusConfig{
				Enabled: false,
				URL:     "http://localhost:9090",
			},
			Logs: LogsConfig{
				Enabled:    false,
				AllowPaths: []string{},
				MaxBytes:   256 * 1024,
				MaxLines:   200,
			},
			Docker: DockerConfig{
				Enabled:  false,
				CLI:      "docker",
				MaxLines: 200,
			},
			Plugins: PluginsConfig{
				Enabled:  false,
				Dir:      "plugins",
				MaxBytes: 256 * 1024,
				Env:      []string{},
			},
		},
	}
}

func LoadConfig(path string) (*NexusConfig, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}

	applyDefaults(cfg)
	return cfg, nil
}

func applyDefaults(cfg *NexusConfig) {
	if cfg.Server.Name == "" {
		cfg.Server.Name = "Nexus"
	}
	if cfg.Server.Version == "" {
		cfg.Server.Version = "v0.0.1"
	}
	if cfg.Server.LogLevel == "" {
		cfg.Server.LogLevel = "info"
	}
	if cfg.Modules.Kubernetes.Kubeconfig == "" {
		cfg.Modules.Kubernetes.Kubeconfig = "~/.kube/config"
	}
	if cfg.Modules.AWS.Region == "" {
		cfg.Modules.AWS.Region = "us-east-1"
	}
	if cfg.Modules.Prometheus.URL == "" {
		cfg.Modules.Prometheus.URL = "http://localhost:9090"
	}
	if cfg.Modules.Logs.MaxBytes <= 0 {
		cfg.Modules.Logs.MaxBytes = 256 * 1024
	}
	if cfg.Modules.Logs.MaxLines <= 0 {
		cfg.Modules.Logs.MaxLines = 200
	}
	if cfg.Modules.Docker.CLI == "" {
		cfg.Modules.Docker.CLI = "docker"
	}
	if cfg.Modules.Docker.MaxLines <= 0 {
		cfg.Modules.Docker.MaxLines = 200
	}
	if cfg.Modules.Plugins.Dir == "" {
		cfg.Modules.Plugins.Dir = "plugins"
	}
	if cfg.Modules.Plugins.MaxBytes <= 0 {
		cfg.Modules.Plugins.MaxBytes = 256 * 1024
	}
}
