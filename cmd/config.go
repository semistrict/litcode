package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

type litcodeConfig struct {
	Docs    []string `json:"docs"`
	Source  []string `json:"source"`
	Exclude []string `json:"exclude"`
}

const configFile = ".litcode.json"

func defaultConfig() litcodeConfig {
	return litcodeConfig{
		Docs: []string{
			"docs/**/*.md",
		},
		Source: []string{
			"**/*.go",
			"**/*.ts",
			"**/*.tsx",
			"**/*.js",
			"**/*.jsx",
			"**/*.py",
			"**/*.rs",
			"**/*.java",
			"**/*.c",
			"**/*.h",
			"**/*.cpp",
			"**/*.hpp",
		},
		Exclude: []string{},
	}
}

func writeConfig(path string, cfg litcodeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

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
