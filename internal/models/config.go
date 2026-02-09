package models

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	InventoryPaths []string `yaml:"inventory_paths"` // ansible inventory paths
	ProfilePath    string   `yaml:"profile_path"`    // wireguard profile path
	AllowedIPs     []string `yaml:"allowed_ips"`     // allowed ips
	ExcludedIPs    []string `yaml:"excluded_ips"`    // excluded ips
	Table          int      `yaml:"table"`           // routing table
	PostUp         []string `yaml:"post_up"`         // post up commands
	PostDown       []string `yaml:"post_down"`       // post down commands
	Debug          bool     `yaml:"debug"`
}

// Read config from file system
func Read(configPath string) (*Config, error) {
	configb, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(configb, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
