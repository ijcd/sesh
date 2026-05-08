// Package spec contains the typed shape of a sesh project YAML file.
package spec

// Project is the resolved, merged, expanded form of a project file.
// Fields use omitempty so unset values round-trip cleanly.
type Project struct {
	Name          string            `yaml:"-"`              // = file basename (set by loader)
	Extends       string            `yaml:"extends,omitempty"`
	Cwd           string            `yaml:"cwd,omitempty"`
	Driver        string            `yaml:"driver,omitempty"`
	Session       string            `yaml:"session,omitempty"`
	Attach        *bool             `yaml:"attach,omitempty"`
	StartupWindow string            `yaml:"startup_window,omitempty"`
	StartupPane   string            `yaml:"startup_pane,omitempty"`
	Vars          map[string]string `yaml:"vars,omitempty"`
	Hooks         Hooks             `yaml:"hooks,omitempty"`
	PreWindow     []string          `yaml:"pre_window,omitempty"`
	Tabs          []Tab             `yaml:"tabs,omitempty"`
}

type Tab struct {
	Title     string   `yaml:"title"`
	Cwd       string   `yaml:"cwd,omitempty"`
	Cmd       string   `yaml:"cmd,omitempty"`
	Driver    string   `yaml:"driver,omitempty"`
	Layout    string   `yaml:"layout,omitempty"`
	PreWindow []string `yaml:"pre_window,omitempty"`
	Panes     []Pane   `yaml:"panes,omitempty"`
	Drop      bool     `yaml:"drop,omitempty"` // sentinel for merge: remove inherited tab
}

type Pane struct {
	Title string `yaml:"title"`
	Cwd   string `yaml:"cwd,omitempty"`
	Cmd   string `yaml:"cmd,omitempty"`
	Drop  bool   `yaml:"drop,omitempty"`
}

type Hooks struct {
	Pre     []string `yaml:"pre,omitempty"`
	Post    []string `yaml:"post,omitempty"`
	OnStart []string `yaml:"on_project_start,omitempty"`
	OnStop  []string `yaml:"on_project_stop,omitempty"`
}
