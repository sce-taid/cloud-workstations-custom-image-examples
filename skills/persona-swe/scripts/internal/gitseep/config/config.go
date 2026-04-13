// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config handles the parsing and validation of GitSeep YAML rules.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Standardized exit codes for CLI integration.
const (
	ExitCodeOK          = 0
	ExitCodeError       = 1
	ExitCodeUsage       = 2
	ExitCodePolicy      = 3
	ExitCodeDataErr     = 65
	ExitCodeNoInput     = 66
	ExitCodeUnavailable = 69
	ExitCodeConfig      = 78
	ExitCodeSigInt      = 130
)

// RulesFileName defines the standard configuration file name.
const RulesFileName = ".gitseep.yaml"

// GlobalConfig contains repository-wide settings for GitSeep.
type GlobalConfig struct {
	BaseRef string `yaml:"base_ref"`
}

// Rule represents a single declarative bedrock definition in the config file.
type Rule struct {
	Date   string
	Branch string
	Parent string
	Paths  []string
}

// GitSeepConfig holds the complete parsed configuration.
type GitSeepConfig struct {
	Global GlobalConfig
	Rules  []Rule
}

// Load parses the GitSeep rules from the specified path or the default local file.
func Load(customPath string) (*GitSeepConfig, error) {
	rulesPath := customPath
	if rulesPath == "" {
		cwd, _ := os.Getwd()
		localRules := filepath.Join(cwd, RulesFileName)
		if _, err := os.Stat(localRules); err == nil {
			rulesPath = localRules
		} else {
			// Try script dir (fallback usually needed if running globally, but for now just error if not found locally)
			return nil, fmt.Errorf("seepage rules not found. Please create %s", RulesFileName)
		}
	}

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	cfg := &GitSeepConfig{}

	if configNode, ok := raw["config"]; ok {
		if configMap, ok := configNode.(map[string]interface{}); ok {
			if baseRef, ok := configMap["base_ref"].(string); ok {
				cfg.Global.BaseRef = baseRef
			}
		}
		delete(raw, "config")
	}

	for key, val := range raw {
		rule := Rule{Date: key}
		switch v := val.(type) {
		case []interface{}:
			for _, p := range v {
				if pathStr, ok := p.(string); ok {
					rule.Paths = append(rule.Paths, pathStr)
				}
			}
		case map[string]interface{}:
			if branch, ok := v["branch"].(string); ok {
				rule.Branch = branch
			}
			if parent, ok := v["parent"].(string); ok {
				rule.Parent = parent
			}
			if paths, ok := v["paths"].([]interface{}); ok {
				for _, p := range paths {
					if pathStr, ok := p.(string); ok {
						rule.Paths = append(rule.Paths, pathStr)
					}
				}
			}
		default:
			return nil, fmt.Errorf("invalid rule format for key %s", key)
		}
		cfg.Rules = append(cfg.Rules, rule)
	}

	return cfg, nil
}
