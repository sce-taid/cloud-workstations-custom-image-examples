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
	"testing"
	"time"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/engine"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
)

func TestDiscoverySuite(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()

	d0 := time.Now()
	h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "base msg", d0)

	t.Run("Standard Percolation Mapping", func(t *testing.T) {
		h1, _ := testutil.CommitFile(repo, wt, "src/owned.txt", "content1", "msg1", d0.Add(time.Hour))
		h2, _ := testutil.CommitFile(repo, wt, "src/owned.txt", "content1_modified", "msg2", d0.Add(time.Hour*2))
		h3, _ := testutil.CommitFile(repo, wt, "other.txt", "content3", "msg3", d0.Add(time.Hour*3))

		seepageCtx := &models.SeepageContext{
			Strata: []string{h1, h2, h3},
			PathToBedrock: map[string]string{
				"src/owned.txt": h1,
			},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr, err := engine.Discover(seepageCtx, repo)
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		if dr.Schedule[h1][h2][0] != "src/owned.txt" {
			t.Errorf("Expected h2 to modify src/owned.txt for h1 bedrock")
		}
		if _, ok := dr.Sources["src/owned.txt"][h2]; !ok {
			t.Errorf("Expected src/owned.txt to list h2 as a source")
		}
		if _, ok := dr.Touched[h3]["other.txt"]; !ok {
			t.Errorf("Expected h3 to touch other.txt")
		}
	})

	t.Run("Path Ownership Overlap (Most Specific Wins)", func(t *testing.T) {
		// Bedrock 1: owns src/
		h1, _ := testutil.CommitFile(repo, wt, "src/main.py", "content1", "msg1", d0.Add(time.Hour*4))

		// Bedrock 2: owns src/core/ (more specific)
		h2, _ := testutil.CommitFile(repo, wt, "src/core/api.py", "content2", "msg2", d0.Add(time.Hour*5))

		// Surface commit touching both
		h3, _ := testutil.CommitFile(repo, wt, "src/main.py", "content1_mod", "msg3", d0.Add(time.Hour*6))
		_, _ = testutil.CommitFile(repo, wt, "src/core/api.py", "content2_mod", "msg4", d0.Add(time.Hour*7))
		h4, _ := repo.Head()

		seepageCtx := &models.SeepageContext{
			Strata: []string{h1, h2, h3, h4.Hash().String()},
			PathToBedrock: map[string]string{
				"src/":      h1,
				"src/core/": h2,
			},
			ParentOfStrata: h0,
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr, err := engine.Discover(seepageCtx, repo)
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		if _, ok := dr.Schedule[h1][h3]; !ok {
			t.Errorf("Expected src/main.py modification to map to h1")
		}
		if _, ok := dr.Schedule[h2][h4.Hash().String()]; !ok {
			t.Errorf("Expected src/core/api.py modification to map to h2, not h1")
		}
		if _, ok := dr.Schedule[h1][h4.Hash().String()]; ok {
			t.Errorf("src/core/api.py incorrectly mapped to the less specific h1 rule")
		}
	})

	t.Run("No-Lithify Policy Violation Check", func(t *testing.T) {
		h1, _ := testutil.CommitFile(repo, wt, "lib/owned.txt", "content1", "msg1", d0.Add(time.Hour*8))
		h2, _ := testutil.CommitFile(repo, wt, "lib/owned.txt", "content1_mod1", "msg2", d0.Add(time.Hour*9))
		h3, _ := testutil.CommitFile(repo, wt, "lib/owned.txt", "content1_mod2", "msg3", d0.Add(time.Hour*10))

		seepageCtx := &models.SeepageContext{
			Strata: []string{h1, h2, h3},
			PathToBedrock: map[string]string{
				"lib/owned.txt": h1,
			},
			Options: models.Options{NoLithify: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr, err := engine.Discover(seepageCtx, repo)
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		// Simulate the engine check for Lithification violation
		lithifyError := false
		for _, sources := range dr.Sources {
			if len(sources) > 1 && seepageCtx.Options.NoLithify {
				lithifyError = true
				break
			}
		}

		if !lithifyError {
			t.Errorf("Expected a Lithification policy violation error")
		}
	})

	t.Run("Empty Strata", func(t *testing.T) {
		seepageCtx := &models.SeepageContext{
			Strata: []string{},
			PathToBedrock: map[string]string{
				"lib/owned.txt": "dummy",
			},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)
		dr, err := engine.Discover(seepageCtx, repo)
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}
		if len(dr.Touched) != 0 {
			t.Errorf("Expected zero touched files")
		}
	})

	t.Run("Concurrency Stress Test", func(t *testing.T) {
		// Generate a larger number of commits to increase the probability of race conditions.
		stressRepo, stressWt := testutil.SetupMemRepo()
		now := time.Now()
		var strata []string
		for i := 0; i < 50; i++ {
			fname := "file.txt"
			content := "content" + string(rune(i))
			h, _ := testutil.CommitFile(stressRepo, stressWt, fname, content, "msg", now.Add(time.Duration(i)*time.Minute))
			strata = append(strata, h)
		}

		seepageCtx := &models.SeepageContext{
			Strata: strata,
			PathToBedrock: map[string]string{
				"file.txt": strata[0],
			},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		// Run Discovery multiple times to further stress the synchronization.
		for i := 0; i < 5; i++ {
			_, err := engine.Discover(seepageCtx, stressRepo)
			if err != nil {
				t.Fatalf("Discover failed during stress test iteration %d: %v", i, err)
			}
		}
	})
}
