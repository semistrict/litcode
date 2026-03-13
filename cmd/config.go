package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

type litcodeConfig struct {
	Docs    []string `json:"docs"`
	Source  []string `json:"source"`
	Exclude []string `json:"exclude,omitempty"`
}

const configFile = ".litcode.json"

func loadConfig() (litcodeConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return litcodeConfig{}, fmt.Errorf("reading %s: %w (run litcode in a directory with a .litcode.json file)", configFile, err)
	}
	var cfg litcodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return litcodeConfig{}, fmt.Errorf("parsing %s: %w", configFile, err)
	}
	if len(cfg.Docs) == 0 {
		return litcodeConfig{}, fmt.Errorf("%s: \"docs\" must be a non-empty array", configFile)
	}
	if len(cfg.Source) == 0 {
		return litcodeConfig{}, fmt.Errorf("%s: \"source\" must be a non-empty array", configFile)
	}
	return cfg, nil
}
