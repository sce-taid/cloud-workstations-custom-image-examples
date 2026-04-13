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

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/engine"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
)

func TestPerformSedimentationInMemory(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()

	d0 := time.Now()
	h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "base msg", d0)

	// Create a new branch "parent_branch" off h0
	_, _ = testutil.CommitFile(repo, wt, "parent.txt", "parent_content", "parent msg", d0.Add(time.Hour))
	parentBranchHash, _ := repo.Head()
	_ = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("parent_branch"), parentBranchHash.Hash()))

	// Go back to h0 to simulate main line
	_ = wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(h0)})

	// The linear commit we want to cherry-pick
	linearH, _ := testutil.CommitFile(repo, wt, "feat.txt", "feat_content", "feat msg", d0.Add(time.Hour*2))

	seepageCtx := &models.SeepageContext{
		DateToBranch: map[string]string{
			"d1": "feat_branch",
		},
		BranchToParent: map[string]string{
			"feat_branch": "parent_branch",
		},
		DateToHash: map[string]string{
			"d1": "bedrock_hash",
		},
		Strata:         []string{"bedrock_hash"},
		ParentOfStrata: h0,
		ResolvedRules: map[string][]string{
			linearH: {"feat.txt"},
		},
		Options: models.Options{DryRun: false},
	}

	orderedBranches := []string{"feat_branch"}
	linearCommits := map[string]string{
		"bedrock_hash": linearH,
	}

	err := engine.PerformSedimentation(seepageCtx, repo, orderedBranches, linearCommits)
	if err != nil {
		t.Fatalf("Sedimentation failed: %v", err)
	}

	// Verify feat_branch was created
	ref, err := repo.Reference(plumbing.NewBranchReferenceName("feat_branch"), true)
	if err != nil {
		t.Fatalf("feat_branch not created")
	}

	// Verify the commit contains both parent.txt and feat.txt
	featCommit, _ := repo.CommitObject(ref.Hash())
	tree, _ := featCommit.Tree()

	_, err = tree.File("parent.txt")
	if err != nil {
		t.Errorf("parent.txt missing from cherry-picked feature branch")
	}

	_, err = tree.File("feat.txt")
	if err != nil {
		t.Errorf("feat.txt missing from cherry-picked feature branch")
	}
}

func TestSedimentationZeroMutation(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()
	d0 := time.Now()
	h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "base msg", d0)

	// The parent branch
	hP, _ := testutil.CommitFile(repo, wt, "parent.txt", "p", "p msg", d0.Add(time.Hour))
	_ = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("parent_branch"), plumbing.NewHash(hP)))

	// Create the feat_branch with a specific tree and message
	_ = wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(hP)})
	hF, _ := testutil.CommitFile(repo, wt, "feat.txt", "f", "f msg", d0.Add(time.Hour*2))
	_ = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("feat_branch"), plumbing.NewHash(hF)))

	seepageCtx := &models.SeepageContext{
		DateToBranch:   map[string]string{"d1": "feat_branch"},
		BranchToParent: map[string]string{"feat_branch": "parent_branch"},
		DateToHash:     map[string]string{"d1": "bedrock_hash"},
		Strata:         []string{"bedrock_hash"},
		ParentOfStrata: h0,
		Options:        models.Options{DryRun: false},
	}

	orderedBranches := []string{"feat_branch"}
	linearCommits := map[string]string{
		"bedrock_hash": hF,
	}

	err := engine.PerformSedimentation(seepageCtx, repo, orderedBranches, linearCommits)
	if err != nil {
		t.Fatalf("Sedimentation failed: %v", err)
	}

	// Check that the branch pointer did NOT move (it still points to hF)
	ref, _ := repo.Reference(plumbing.NewBranchReferenceName("feat_branch"), true)
	if ref.Hash().String() != hF {
		t.Errorf("Expected branch to not be mutated, but it moved from %s to %s", hF, ref.Hash().String())
	}
}

func TestDateBasedImplicitParent(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()
	d := time.Now()
	hBase, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d)

	// Create a feature branch for an OLD bedrock
	hA, _ := testutil.CommitFile(repo, wt, "a.txt", "a", "msgA", d.Add(time.Hour))
	_ = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("feat/A"), plumbing.NewHash(hA)))

	// Feature branch for a NEWER bedrock
	hB, _ := testutil.CommitFile(repo, wt, "b.txt", "b", "msgB", d.Add(time.Hour*2))

	seepageCtx := &models.SeepageContext{
		DateToBranch: map[string]string{
			"2026-04-13": "feat/A",
			"2026-04-14": "feat/B",
		},
		DateToHash: map[string]string{
			"2026-04-13": hA,
			"2026-04-14": hB,
		},
		BranchToParent: map[string]string{
			// All implicit
		},
		Strata:         []string{hA, hB},
		ParentOfStrata: hBase,
		Options:        models.Options{DryRun: false},
	}

	// We want to verify that feat/B correctly identifies feat/A as its parent
	// based on the date, even if not explicitly told.
	err := engine.PerformSedimentation(seepageCtx, repo, []string{"feat/A", "feat/B"}, map[string]string{hA: hA, hB: hB})
	if err != nil {
		t.Fatalf("Sedimentation failed: %v", err)
	}

	refB, _ := repo.Reference(plumbing.NewBranchReferenceName("feat/B"), true)
	commitB, _ := repo.CommitObject(refB.Hash())

	if commitB.NumParents() == 0 || commitB.ParentHashes[0].String() != hA {
		t.Errorf("feat/B did not correctly use feat/A as its parent")
	}
}

func TestSharedHistoryOptimization(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()
	d0 := time.Now()
	hBase, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "msg0", d0)

	// Linear chain: Base -> L1 -> L2
	hL1, _ := testutil.CommitFile(repo, wt, "a.txt", "a", "msg1", d0.Add(time.Hour))
	hL2, _ := testutil.CommitFile(repo, wt, "b.txt", "b", "msg2", d0.Add(time.Hour*2))

	seepageCtx := &models.SeepageContext{
		DateToBranch: map[string]string{
			"d1": "feat/A",
			"d2": "feat/B",
		},
		DateToHash: map[string]string{
			"d1": hL1,
			"d2": hL2,
		},
		BranchToParent: map[string]string{
			"feat/A": "",       // implicit base
			"feat/B": "feat/A", // explicit parent
		},
		Strata:         []string{hL1, hL2},
		ParentOfStrata: hBase,
		Options:        models.Options{DryRun: false},
	}

	// We'll use the linear hashes themselves as the reconstructed commits
	linearCommits := map[string]string{
		hL1: hL1,
		hL2: hL2,
	}

	err := engine.PerformSedimentation(seepageCtx, repo, []string{"feat/A", "feat/B"}, linearCommits)
	if err != nil {
		t.Fatalf("Sedimentation failed: %v", err)
	}

	// Verify feat/A uses hL1 exactly (since both have hBase as parent)
	refA, _ := repo.Reference(plumbing.NewBranchReferenceName("feat/A"), true)
	if refA.Hash().String() != hL1 {
		t.Errorf("feat/A did not reuse linear commit hash L1")
	}

	// Verify feat/B uses hL2 exactly (since feat/B's parent was set to feat/A which is hL1, and L2's parent is L1)
	refB, _ := repo.Reference(plumbing.NewBranchReferenceName("feat/B"), true)
	if refB.Hash().String() != hL2 {
		t.Errorf("feat/B did not reuse linear commit hash L2")
	}
}
