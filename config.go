package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Defaults  DefaultsConfig   `yaml:"defaults"`
	Instances []InstanceConfig `yaml:"instances"`
}

type ServerConfig struct {
	Token string `yaml:"token"`
	Port  int    `yaml:"port"`
}

type DefaultsConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds"`
	MaxRows        int `yaml:"max_rows"`
}

type InstanceConfig struct {
	InstanceID     string `yaml:"instance_id"`
	DSN            string `yaml:"dsn"`
	Environment    string `yaml:"environment"`
	ReadOnly       bool   `yaml:"read_only"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	MaxRows        int    `yaml:"max_rows"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Instances) == 0 {
		return fmt.Errorf("config: at least one instance is required")
	}

	if c.Defaults.TimeoutSeconds <= 0 {
		c.Defaults.TimeoutSeconds = 30
	}
	if c.Defaults.MaxRows <= 0 {
		c.Defaults.MaxRows = 10000
	}

	seen := make(map[string]bool)
	for i, inst := range c.Instances {
		if inst.InstanceID == "" {
			return fmt.Errorf("config: instance[%d] must have an instance_id", i)
		}
		if inst.DSN == "" {
			return fmt.Errorf("config: instance[%d] (%s) must have a dsn", i, inst.InstanceID)
		}
		if seen[inst.InstanceID] {
			return fmt.Errorf("config: duplicate instance_id %q", inst.InstanceID)
		}
		seen[inst.InstanceID] = true

		if inst.TimeoutSeconds <= 0 {
			c.Instances[i].TimeoutSeconds = c.Defaults.TimeoutSeconds
		}
		if inst.TimeoutSeconds > c.Defaults.TimeoutSeconds {
			return fmt.Errorf("config: instance[%d] (%s) timeout (%ds) exceeds default max (%ds)",
				i, inst.InstanceID, inst.TimeoutSeconds, c.Defaults.TimeoutSeconds)
		}
		if inst.MaxRows <= 0 {
			c.Instances[i].MaxRows = c.Defaults.MaxRows
		}
		if inst.MaxRows > c.Defaults.MaxRows {
			return fmt.Errorf("config: instance[%d] (%s) max_rows (%d) exceeds default max (%d)",
				i, inst.InstanceID, inst.MaxRows, c.Defaults.MaxRows)
		}
	}

	return nil
}
