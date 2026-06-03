// Package config loads sesh project YAML files into spec.Project values.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/ijcd/sesh/internal/spec"
	"github.com/ijcd/sesh/internal/xdg"
)

// LoadFile reads a single YAML file from disk and returns its decoded form.
// Project.Name is set to the file basename (without extension).
// No extends/merge/vars/validate is applied — that's Config.Load's job.
func LoadFile(path string) (*spec.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var p spec.Project
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	p.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &p, nil
}

// ResolveProjectPath returns the absolute path of a project file given its name.
// $XDG_CONFIG_HOME/sesh/projects/<name>.yml, or ~/.config/sesh/projects/<name>.yml.
func ResolveProjectPath(name string) (string, error) {
	base, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "projects", name+".yml"), nil
}

// ResolveTemplatePath returns the absolute path of a template file given its name
// or relative path. Paths starting with './' or '../' resolve relative to baseDir.
// Absolute paths are returned as-is. All other names (including subdirectory names
// like "hooks/direnv") resolve to <config>/templates/<name>.yml.
func ResolveTemplatePath(nameOrPath, baseDir string) (string, error) {
	if strings.HasPrefix(nameOrPath, "./") || strings.HasPrefix(nameOrPath, "../") {
		return filepath.Clean(filepath.Join(baseDir, nameOrPath)), nil
	}
	if filepath.IsAbs(nameOrPath) {
		return nameOrPath, nil
	}
	base, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "templates", nameOrPath+".yml"), nil
}

func configDir() (string, error) {
	cfg, err := xdg.ConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "sesh"), nil
}
