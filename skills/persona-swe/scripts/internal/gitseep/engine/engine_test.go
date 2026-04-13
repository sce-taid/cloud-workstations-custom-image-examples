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

package engine

import (
	"testing"
	"time"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/config"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
)

func TestEngineHelpers(t *testing.T) {
	t.Run("indexOf", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		if i := models.IndexOf(slice, "b"); i != 1 {
			t.Errorf("Expected index 1, got %d", i)
		}
		if i := models.IndexOf(slice, "z"); i != -1 {
			t.Errorf("Expected index -1, got %d", i)
		}
	})

	t.Run("parseRuleDate", func(t *testing.T) {
		dateStr := "2026-04-13 10:01:55 +0000"
		tm := parseRuleDate(dateStr)
		if tm.IsZero() {
			t.Fatalf("Failed to parse full date")
		}
		if tm.Year() != 2026 {
			t.Errorf("Expected year 2026, got %d", tm.Year())
		}

		dateStrShort := "2026-04-13"
		tm = parseRuleDate(dateStrShort)
		if tm.IsZero() {
			t.Fatalf("Failed to parse short date")
		}
		if tm.Month() != time.April {
			t.Errorf("Expected month April, got %v", tm.Month())
		}
	})
}

func TestNewContext(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()
	d0 := time.Now()
	h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d0)
	h1, _ := testutil.CommitFile(repo, wt, "file1.txt", "content1", "msg1", d0.Add(time.Hour))

	dateStr := d0.Add(time.Hour).Format("2006-01-02 15:04:05 -0700")
	cfg := &config.GitSeepConfig{
		Global: config.GlobalConfig{BaseRef: "master"},
		Rules: []config.Rule{
			{
				Date:  dateStr,
				Paths: []string{"file1.txt"},
			},
		},
	}

	opts := models.Options{
		BaseCommit: h1,
	}

	seepageCtx, err := NewContext(repo, cfg, opts)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if seepageCtx.ParentOfStrata != h0 {
		t.Errorf("Expected ParentOfStrata %s, got %s", h0, seepageCtx.ParentOfStrata)
	}
	if len(seepageCtx.Strata) != 1 || seepageCtx.Strata[0] != h1 {
		t.Errorf("Expected strata [h1], got %v", seepageCtx.Strata)
	}
}
