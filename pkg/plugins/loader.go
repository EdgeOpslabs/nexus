package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ManifestName = "nexus.yaml"

func LoadManifests(dir string) ([]Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var manifests []Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), ManifestName)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var manifest Manifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		if manifest.Metadata.Name == "" {
			manifest.Metadata.Name = entry.Name()
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}
