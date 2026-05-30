// Package spec contains the typed shape of a sesh project YAML file.
package spec

import "github.com/goccy/go-yaml"

// StringList accepts either a YAML scalar or a YAML sequence and stores both as []string.
type StringList []string

func (s *StringList) UnmarshalYAML(b []byte) error {
	var single string
	if err := yaml.Unmarshal(b, &single); err == nil {
		*s = StringList{single}
		return nil
	}
	var multi []string
	if err := yaml.Unmarshal(b, &multi); err != nil {
		return err
	}
	*s = StringList(multi)
	return nil
}

// Project is the resolved, merged, expanded form of a project file.
// Fields use omitempty so unset values round-trip cleanly.
type Project struct {
	Name          string            `yaml:"-"` // = file basename (set by loader)
	Include       StringList        `yaml:"include,omitempty"`
	Extends       string            `yaml:"extends,omitempty"` // deprecated; validate.go errors if set
	Cwd           string            `yaml:"cwd,omitempty"`
	Driver        string            `yaml:"driver,omitempty"`
	Session       string            `yaml:"session,omitempty"`
	Attach        *bool             `yaml:"attach,omitempty"`
	StartupWindow string            `yaml:"startup_window,omitempty"`
	StartupPane   string            `yaml:"startup_pane,omitempty"`
	Vars          map[string]string `yaml:"vars,omitempty"`
	Hooks         Hooks             `yaml:"hooks,omitempty"`
	PreWindow     StringList        `yaml:"pre_window,omitempty"`
	Tabs          []Tab             `yaml:"tabs,omitempty"`
	Apps          []App             `yaml:"apps,omitempty"`
}

// UnmarshalYAML decodes a Project via the default field-tag path, then
// normalizes the apps[] list (auto-indexing absent IDs per plugin).
func (p *Project) UnmarshalYAML(b []byte) error {
	type projectAlias Project
	var raw projectAlias
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = Project(raw)
	p.normalizeApps()
	return nil
}

type Tab struct {
	Title     string     `yaml:"title"`
	Cwd       string     `yaml:"cwd,omitempty"`
	Cmd       string     `yaml:"cmd,omitempty"`
	Driver    string     `yaml:"driver,omitempty"`
	Layout    string     `yaml:"layout,omitempty"`
	PreWindow StringList `yaml:"pre_window,omitempty"`
	Panes     []Pane     `yaml:"panes,omitempty"`
	Drop      bool       `yaml:"drop,omitempty"` // sentinel for merge: remove inherited tab
}

type Pane struct {
	Title string `yaml:"title"`
	Cwd   string `yaml:"cwd,omitempty"`
	Cmd   string `yaml:"cmd,omitempty"`
	Drop  bool   `yaml:"drop,omitempty"`
}

type Hooks struct {
	Pre     StringList `yaml:"pre,omitempty"`
	Post    StringList `yaml:"post,omitempty"`
	OnStart StringList `yaml:"on_project_start,omitempty"`
	OnStop  StringList `yaml:"on_project_stop,omitempty"`
}
