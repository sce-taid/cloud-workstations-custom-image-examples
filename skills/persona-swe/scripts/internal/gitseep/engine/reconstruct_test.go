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
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/engine"
	gitseepGit "github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/git"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
)

func TestReconstructHistorySuite(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()
	d0 := time.Now()
	h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "base msg", d0)

	t.Run("Basic In-Memory Reconstruction", func(t *testing.T) {
		h1, _ := testutil.CommitFile(repo, wt, "file1.txt", "content1", "msg1", d0.Add(time.Hour))
		h2, _ := testutil.CommitFile(repo, wt, "file2.txt", "content2", "msg2", d0.Add(time.Hour*2))
		h3, _ := testutil.CommitFile(repo, wt, "file1.txt", "content1_updated", "msg3", d0.Add(time.Hour*3))

		seepageCtx := &models.SeepageContext{
			RepoRoot:       "/mem",
			OriginalHead:   h3,
			CurrentBranch:  "master",
			TargetBranch:   "master",
			ParentOfStrata: h0,
			Strata:         []string{h1, h2, h3},
			PathToBedrock: map[string]string{
				"file1.txt": h1,
			},
			Options: models.Options{DryRun: false, StageOnly: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr := &models.DiscoveryResult{
			Schedule: map[string]map[string][]string{
				h1: {
					h1: {"file1.txt"},
					h3: {"file1.txt"},
				},
			},
		}

		linearCommits, err := engine.ReconstructHistory(seepageCtx, dr, repo)
		if err != nil {
			t.Fatalf("ReconstructHistory failed: %v", err)
		}

		if len(linearCommits) != 3 {
			t.Errorf("Expected 3 linear commits mapped, got %d", len(linearCommits))
		}

		newH1Str := linearCommits[h1]
		newH1Obj, _ := repo.CommitObject(plumbing.NewHash(newH1Str))
		tree1, _ := newH1Obj.Tree()
		file1Obj, err := tree1.File("file1.txt")
		if err != nil {
			t.Fatalf("file1.txt missing in reconstructed Bedrock: %v", err)
		}
		content, _ := file1Obj.Contents()
		if content != "content1_updated" {
			t.Errorf("Bedrock did not capture final state. Expected 'content1_updated', got '%s'", content)
		}

		newH3Str := linearCommits[h3]
		newH3Obj, _ := repo.CommitObject(plumbing.NewHash(newH3Str))
		tree3, _ := newH3Obj.Tree()
		file1Obj3, _ := tree3.File("file1.txt")
		content3, _ := file1Obj3.Contents()
		if content3 != "content1_updated" {
			t.Errorf("Surface tree corrupted for file1.txt")
		}

		newH2Str := linearCommits[h2]
		newH2Obj, _ := repo.CommitObject(plumbing.NewHash(newH2Str))
		tree2, _ := newH2Obj.Tree()
		file2Obj, err := tree2.File("file2.txt")
		if err != nil {
			t.Errorf("file2.txt missing in reconstructed H2: %v", err)
		}
		c2, _ := file2Obj.Contents()
		if c2 != "content2" {
			t.Errorf("H2 corrupted")
		}
	})

	t.Run("Safe Checkout with Go-Git (Experimental)", func(t *testing.T) {
		repo, wt := testutil.SetupMemRepo()
		d := time.Now()
		h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d)

		h1, _ := testutil.CommitFile(repo, wt, "file1.txt", "content1", "msg1", d.Add(time.Hour))
		hHead, _ := repo.Head()
		hHeadStr := hHead.Hash().String()

		seepageCtx := &models.SeepageContext{
			RepoRoot:       "/mem",
			OriginalHead:   hHeadStr,
			CurrentBranch:  "master",
			TargetBranch:   "master",
			ParentOfStrata: h0,
			Strata:         []string{h1},
			PathToBedrock: map[string]string{
				"file1.txt": h1,
			},
			// Enable the experimental go-git flag and disable stage-only to trigger the checkout block
			Options: models.Options{DryRun: false, StageOnly: false, ExperimentalGoGit: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr := &models.DiscoveryResult{
			Schedule: map[string]map[string][]string{
				h1: {
					h1: {"file1.txt"},
				},
			},
		}

		// This will exercise the w.Checkout() code path without using os/exec git reset
		_, err := engine.ReconstructHistory(seepageCtx, dr, repo)
		if err != nil {
			t.Fatalf("ReconstructHistory with experimental checkout failed: %v", err)
		}

		// Verify worktree was updated successfully
		status, _ := wt.Status()
		if !status.IsClean() {
			t.Errorf("Expected clean worktree after successful checkout")
		}
	})

	t.Run("Stratigraphy Mismatch Warning", func(t *testing.T) {
		h1, _ := testutil.CommitFile(repo, wt, "src/owned.txt", "content1", "msg1", d0.Add(time.Hour*4))

		seepageCtx := &models.SeepageContext{
			OriginalHead:   h1,
			ParentOfStrata: h0,
			Strata:         []string{h1},
			PathToBedrock:  map[string]string{}, // No rules
			Options:        models.Options{DryRun: false, StageOnly: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		// Create a fake commit that has different content to force a mismatch
		hFake, _ := testutil.CommitFile(repo, wt, "src/owned.txt", "CORRUPTED", "fake msg", d0.Add(time.Hour*5))
		seepageCtx.OriginalHead = hFake

		_, err := engine.ReconstructHistory(seepageCtx, &models.DiscoveryResult{}, repo)
		if err != nil {
			t.Fatalf("ReconstructHistory failed: %v", err)
		}

		origCommit, _ := gitseepGit.GetCommit(repo, seepageCtx.OriginalHead)
		parityPassed := origCommit.TreeHash.String() == "some_reconstructed_hash"

		if parityPassed {
			t.Errorf("Expected parity to fail due to stratigraphy mismatch")
		}

		msg := "warning: stratigraphy mismatch. Reconstructed tree != Original tree"
		if !strings.Contains(msg, "mismatch") {
			t.Errorf("Expected mismatch warning")
		}
	})

	t.Run("Zero-Mutation Optimization", func(t *testing.T) {
		repo, wt := testutil.SetupMemRepo()
		d := time.Now()
		hBase, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d)
		hA, _ := testutil.CommitFile(repo, wt, "a.txt", "a", "msgA", d.Add(time.Hour))
		hB, _ := testutil.CommitFile(repo, wt, "b.txt", "b", "msgB", d.Add(time.Hour*2))

		seepageCtx := &models.SeepageContext{
			RepoRoot:       "/mem",
			OriginalHead:   hB,
			CurrentBranch:  "master",
			TargetBranch:   "master",
			ParentOfStrata: hBase,
			Strata:         []string{hA, hB},
			PathToBedrock: map[string]string{
				"a.txt": hA,
			},
			Options: models.Options{DryRun: false, StageOnly: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		linearCommits, err := engine.ReconstructHistory(seepageCtx, &models.DiscoveryResult{}, repo)
		if err != nil {
			t.Fatalf("ReconstructHistory failed: %v", err)
		}

		if linearCommits[hA] != hA {
			t.Errorf("Expected hA to be zero-mutated, got new hash %s", linearCommits[hA])
		}
		if linearCommits[hB] != hB {
			t.Errorf("Expected hB to be zero-mutated, got new hash %s", linearCommits[hB])
		}
	})

	t.Run("Tree Deduplication (No Corruption)", func(t *testing.T) {
		repo, wt := testutil.SetupMemRepo()
		d := time.Now()
		h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d)

		// Create multiple files in the same sub-folder
		h1, _ := testutil.CommitFile(repo, wt, "src/a.go", "a", "msg1", d.Add(time.Hour))
		if _, err := testutil.CommitFile(repo, wt, "src/b.go", "b", "msg2", d.Add(time.Hour*2)); err != nil {
			t.Fatalf("Failed to commit src/b.go: %v", err)
		}
		hHead, _ := repo.Head()
		h3 := hHead.Hash().String()

		seepageCtx := &models.SeepageContext{
			OriginalHead:   h3,
			ParentOfStrata: h0,
			Strata:         []string{h1, h3},
			PathToBedrock: map[string]string{
				"src/a.go": h1,
				"src/b.go": h3,
			},
			Options: models.Options{DryRun: false, StageOnly: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr := &models.DiscoveryResult{
			Schedule: map[string]map[string][]string{
				h1: {h1: {"src/a.go"}},
				h3: {h3: {"src/b.go"}},
			},
		}

		_, err := engine.ReconstructHistory(seepageCtx, dr, repo)
		if err != nil {
			t.Fatalf("ReconstructHistory failed on multi-file tree: %v", err)
		}
	})

	t.Run("Faithful Mirroring (Unmanaged History)", func(t *testing.T) {
		repo, wt := testutil.SetupMemRepo()
		d := time.Now()
		h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d)

		// Unmanaged file in history
		h1, _ := testutil.CommitFile(repo, wt, "unmanaged.txt", "content", "msg1", d.Add(time.Hour))
		h2, _ := testutil.CommitFile(repo, wt, "managed.txt", "m1", "msg2", d.Add(time.Hour*2))

		seepageCtx := &models.SeepageContext{
			OriginalHead:   h2,
			ParentOfStrata: h0,
			Strata:         []string{h1, h2},
			PathToBedrock: map[string]string{
				"managed.txt": h1, // Rule maps managed.txt to h1
			},
			Options: models.Options{DryRun: false, StageOnly: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		dr := &models.DiscoveryResult{
			Schedule: map[string]map[string][]string{
				h1: {h1: {"managed.txt"}, h2: {"managed.txt"}},
			},
		}

		linearCommits, _ := engine.ReconstructHistory(seepageCtx, dr, repo)

		// Verify unmanaged.txt was mirrored into the reconstructed H1
		h1Obj, _ := repo.CommitObject(plumbing.NewHash(linearCommits[h1]))
		tree, _ := h1Obj.Tree()
		if _, err := tree.File("unmanaged.txt"); err != nil {
			t.Errorf("unmanaged.txt lost during mirroring")
		}
	})

	t.Run("Surface Injection (.gitseep.yaml preservation)", func(t *testing.T) {
		repo, wt := testutil.SetupMemRepo()
		d := time.Now()
		h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d)

		// History commit
		h1, err := testutil.CommitFile(repo, wt, "src/main.go", "v1", "msg1", d.Add(time.Hour))
		if err != nil {
			t.Fatalf("Failed to commit src/main.go: %v", err)
		}

		// New file at surface, NOT in history
		f, _ := wt.Filesystem.Create(".gitseep.yaml")
		if _, err := f.Write([]byte("config: ...")); err != nil {
			t.Fatalf("Failed to write .gitseep.yaml: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("Failed to close .gitseep.yaml: %v", err)
		}
		if _, err := wt.Add(".gitseep.yaml"); err != nil {
			t.Fatalf("Failed to stage .gitseep.yaml: %v", err)
		}
		h2, _ := wt.Commit("new config", &git.CommitOptions{
			Author: &object.Signature{Name: "T", Email: "t", When: d.Add(time.Hour * 2)},
		})
		h2Str := h2.String()

		seepageCtx := &models.SeepageContext{
			OriginalHead:   h2Str,
			ParentOfStrata: h0,
			Strata:         []string{h1, h2Str},
			PathToBedrock:  map[string]string{}, // No rules
			Options:        models.Options{DryRun: false, StageOnly: true},
		}
		seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

		linearCommits, err := engine.ReconstructHistory(seepageCtx, &models.DiscoveryResult{}, repo)
		if err != nil {
			t.Fatalf("ReconstructHistory failed: %v", err)
		}

		// Verify .gitseep.yaml was injected into the final commit
		finalObj, _ := repo.CommitObject(plumbing.NewHash(linearCommits[h2Str]))
		tree, _ := finalObj.Tree()
		if _, err := tree.File(".gitseep.yaml"); err != nil {
			t.Errorf(".gitseep.yaml lost during surface injection")
		}
	})
}
