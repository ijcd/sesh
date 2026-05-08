// Package config loads sesh project YAML files into spec.Project values.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/ijcd/sesh/internal/spec"
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
// or relative path. Names containing '/' or starting with './' resolve relative
// to baseDir. Bare names resolve to <config>/templates/<name>.yml.
func ResolveTemplatePath(nameOrPath, baseDir string) (string, error) {
	if strings.HasPrefix(nameOrPath, "./") || strings.HasPrefix(nameOrPath, "../") || strings.Contains(nameOrPath, "/") {
		if filepath.IsAbs(nameOrPath) {
			return nameOrPath, nil
		}
		return filepath.Clean(filepath.Join(baseDir, nameOrPath)), nil
	}
	base, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "templates", nameOrPath+".yml"), nil
}

func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "sesh"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "sesh"), nil
}
