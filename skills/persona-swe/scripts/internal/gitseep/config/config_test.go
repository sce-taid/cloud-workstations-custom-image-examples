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

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/config"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantBaseRef string
		wantRules   int
		checkRules  func(t *testing.T, cfg *config.GitSeepConfig)
	}{
		{
			name: "Standard Multi-Path Rule",
			yaml: `
"2026-04-15":
  - examples/preflight/
  - examples/images/gnome/
`,
			wantRules: 1,
			checkRules: func(t *testing.T, cfg *config.GitSeepConfig) {
				if len(cfg.Rules[0].Paths) != 2 {
					t.Errorf("expected 2 paths, got %d", len(cfg.Rules[0].Paths))
				}
			},
		},
		{
			name: "Advanced Stacked PR Rule",
			yaml: `
config:
  base_ref: origin/main

"2026-04-16 10:00:00 +0000":
  branch: feat/gnome
  paths:
    - examples/images/gnome/

"2026-04-17 12:00:00 +0000":
  branch: feat/asfp
  parent: feat/gnome
  paths:
    - examples/images/android-studio-for-platform/
`,
			wantBaseRef: "origin/main",
			wantRules:   2,
			checkRules: func(t *testing.T, cfg *config.GitSeepConfig) {
				for _, r := range cfg.Rules {
					if r.Date == "2026-04-17 12:00:00 +0000" {
						if r.Branch != "feat/asfp" || r.Parent != "feat/gnome" {
							t.Errorf("incorrect branch/parent for asfp rule")
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			rulesPath := filepath.Join(tmpDir, ".gitseep.yaml")
			if err := os.WriteFile(rulesPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write mock config: %v", err)
			}

			cfg, err := config.Load(rulesPath)
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			if cfg.Global.BaseRef != tt.wantBaseRef {
				t.Errorf("expected base_ref %q, got %q", tt.wantBaseRef, cfg.Global.BaseRef)
			}

			if len(cfg.Rules) != tt.wantRules {
				t.Errorf("expected %d rules, got %d", tt.wantRules, len(cfg.Rules))
			}

			if tt.checkRules != nil {
				tt.checkRules(t, cfg)
			}
		})
	}
}
