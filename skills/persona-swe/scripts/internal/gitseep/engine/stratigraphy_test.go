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

package engine_test

import (
	"strings"
	"testing"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/engine"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

func TestStratigraphyValidationCycle(t *testing.T) {
	seepageCtx := &models.SeepageContext{
		DateToBranch: map[string]string{
			"d1": "branch_A",
			"d2": "branch_B",
		},
		BranchToParent: map[string]string{
			"branch_A": "branch_B",
			"branch_B": "branch_A", // Circular dependency
		},
		Strata:        []string{"h1", "h2"},
		ResolvedRules: map[string][]string{},
	}
	seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

	_, err := engine.ValidateStratigraphyAndPredictConflicts(seepageCtx, &models.DiscoveryResult{})
	if err == nil {
		t.Fatalf("Expected cycle detection error, got nil")
	}

	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestImplicitParentResolution(t *testing.T) {
	seepageCtx := &models.SeepageContext{
		DateToBranch: map[string]string{
			"2026-04-14 17:36:44 +0000": "feat/C",
			"2026-04-13 12:24:06 +0000": "feat/B",
			"2025-08-29 07:51:48 +0000": "feat/A", // Oldest
		},
		BranchToParent: map[string]string{
			// All implicit
		},
		// Deliberately omit these from Strata to prove we don't rely on it
		Strata:        []string{},
		ResolvedRules: map[string][]string{},
	}
	seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

	orderedBranches, err := engine.ValidateStratigraphyAndPredictConflicts(seepageCtx, &models.DiscoveryResult{})
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if len(orderedBranches) != 3 {
		t.Fatalf("Expected 3 ordered branches, got %d", len(orderedBranches))
	}

	if orderedBranches[0] != "feat/A" {
		t.Errorf("Expected feat/A (oldest) to be first, got %s", orderedBranches[0])
	}
	if orderedBranches[1] != "feat/B" {
		t.Errorf("Expected feat/B to be second, got %s", orderedBranches[1])
	}
	if orderedBranches[2] != "feat/C" {
		t.Errorf("Expected feat/C (newest) to be third, got %s", orderedBranches[2])
	}
}

func TestMathematicalConflictPrediction(t *testing.T) {
	seepageCtx := &models.SeepageContext{
		DateToBranch: map[string]string{
			"d_S":  "branch_S",
			"d_PS": "branch_P",
		},
		BranchToParent: map[string]string{
			"branch_S": "branch_P",
		},
		DateToHash: map[string]string{
			"d_S":  "S",
			"d_PS": "P_S",
		},
		Strata: []string{"P_S", "K", "S"}, // P_S is target, K is skipped, S is source
	}
	seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

	t.Run("Actual Conflict", func(t *testing.T) {
		// S modifies file.txt, K also modifies file.txt
		// file.txt is NOT owned by S (it's foreign)
		dr := &models.DiscoveryResult{
			Touched: map[string]map[string]struct{}{
				"S": {"file.txt": {}},
				"K": {"file.txt": {}},
			},
		}
		seepageCtx.PathToBedrock = map[string]string{} // No one owns file.txt
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		_, err := engine.ValidateStratigraphyAndPredictConflicts(seepageCtx, dr)
		if err == nil {
			t.Fatalf("Expected conflict prediction error, got nil")
		}
		if !strings.Contains(err.Error(), "conflict predicted") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Lithification (Suppressed Conflict)", func(t *testing.T) {
		// S modifies .gitignore, K also modifies .gitignore
		// BUT .gitignore is owned by S bedrock.
		dr := &models.DiscoveryResult{
			Touched: map[string]map[string]struct{}{
				"S": {".gitignore": {}},
				"K": {".gitignore": {}},
			},
		}
		seepageCtx.PathToBedrock = map[string]string{
			".gitignore": "S",
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		_, err := engine.ValidateStratigraphyAndPredictConflicts(seepageCtx, dr)
		if err != nil {
			t.Fatalf("Expected NO conflict due to lithification filtering, got: %v", err)
		}
	})
}
