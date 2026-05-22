package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// UpsertStrategyInstance replaces or appends an instance by ID in config.yaml.
func UpsertStrategyInstance(configPath string, inst StrategyInstanceConfig) error {
	if strings.TrimSpace(inst.ID) == "" {
		return fmt.Errorf("instance id is required")
	}
	v := viper.New()
	if configPath == "" {
		configPath = "config/config.yaml"
	}
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	found := false
	for i := range cfg.Strategies.Instances {
		if cfg.Strategies.Instances[i].ID == inst.ID {
			cfg.Strategies.Instances[i] = inst
			found = true
			break
		}
	}
	if !found {
		cfg.Strategies.Instances = append(cfg.Strategies.Instances, inst)
	}
	v.Set("strategies.instances", cfg.Strategies.Instances)
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
