package plugins

type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name        string `yaml:"name"`
	Vendor      string `yaml:"vendor"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

type Spec struct {
	Command      string            `yaml:"command"`
	Args         []string          `yaml:"args"`
	Env          map[string]string `yaml:"env"`
	Capabilities Capabilities      `yaml:"capabilities"`
}

type Capabilities struct {
	Tools []ToolSpec `yaml:"tools"`
}

type ToolSpec struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	ReadOnly    bool      `yaml:"read_only"`
	Args        []ArgSpec `yaml:"args"`
}

type ArgSpec struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // string, number, integer, boolean
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}
