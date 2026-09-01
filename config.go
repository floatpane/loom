package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// loomConfig holds user-configurable settings, persisted to
// ~/.loom/config.json. All fields are optional; zero values mean
// "use defaults".
type loomConfig struct {
	// Signoff indicates whether to append a Signed-off-by trailer by default.
	Signoff bool `json:"signoff,omitempty"`
	// CoAuthor is a list of "Name <email>" strings always available as
	// quick-add co-authors.
	CoAuthor []string `json:"coAuthors,omitempty"`
}

func configFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".loom", "config.json")
}

func loadLoomConfig() *loomConfig {
	cfg := &loomConfig{}
	path := configFilePath()
	if path == "" {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	return cfg
}

func (cfg *loomConfig) save() {
	path := configFilePath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, data, 0644)
}
