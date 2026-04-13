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
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/gitutil"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

// PerformSedimentation synchronizes the perfectly refactored Bedrock commits from the unified linear history
// back into their respective isolated feature branches, autonomously building a Stacked PR DAG.
func PerformSedimentation(seepageCtx *models.SeepageContext, repo *git.Repository, orderedBranches []string, linearCommits map[string]string) error {
	if len(orderedBranches) == 0 {
		return nil
	}

	logger.Info("\n--- Phase 4: %s Sedimentation (autonomous branch sync) ---", logger.IconBranch)

	branchHeads := make(map[string]plumbing.Hash)

	for _, branchName := range orderedBranches {
		var bedrockH string
		var newCommitHash plumbing.Hash
		for d, b := range seepageCtx.DateToBranch {
			if b == branchName {
				bedrockH = seepageCtx.DateToHash[d]
				break
			}
		}

		if bedrockH == "" {
			continue
		}

		// Find the hash in the reconstructed linear history
		var linearH plumbing.Hash
		for s, h := range linearCommits {
			if s == bedrockH {
				linearH = plumbing.NewHash(h)
				break
			}
		}

		if linearH.IsZero() {
			continue
		}

		linearCommit, err := repo.CommitObject(linearH)
		if err != nil {
			logger.Error("      ⮑  Failed to read linear commit: %v", err)
			continue
		}

		// Resolve topological parent
		parentName := seepageCtx.BranchToParent[branchName]
		if parentName == "" {
			// If not explicit, try implicit chronological parent
			var dates []string
			for d := range seepageCtx.DateToBranch {
				dates = append(dates, d)
			}
			sort.Slice(dates, func(i, j int) bool {
				tI := parseRuleDate(dates[i])
				tJ := parseRuleDate(dates[j])
				if !tI.IsZero() && !tJ.IsZero() {
					return tI.Before(tJ)
				}
				return dates[i] < dates[j]
			})

			for d, b := range seepageCtx.DateToBranch {
				if b == branchName {
					idx := models.IndexOf(dates, d)
					if idx > 0 {
						parentName = seepageCtx.DateToBranch[dates[idx-1]]
					}
					break
				}
			}
		}

		var targetParentHash plumbing.Hash
		if parentName != "" {
			if h, ok := branchHeads[parentName]; ok {
				targetParentHash = h
			} else {
				// Try to resolve in repo
				ref, err := repo.Reference(plumbing.NewBranchReferenceName(parentName), true)
				if err == nil {
					targetParentHash = ref.Hash()
				}
			}
		}

		// LINEAR PARENT INHERITANCE:
		// If no explicit parent is defined, or we failed to resolve it,
		// we default to the parent of the linear commit.
		// This ensures that unassigned intermediate commits are inherited,
		// allowing for shared history hashes.
		if targetParentHash.IsZero() {
			if linearCommit.NumParents() > 0 {
				targetParentHash = linearCommit.ParentHashes[0]
			} else {
				targetParentHash = plumbing.NewHash(seepageCtx.ParentOfStrata)
			}
		}

		logger.Info("  [%s] Syncing %s (onto %s)", logger.IconBranch, logger.StyleBold.Render(branchName), logger.ColorHash(targetParentHash.String()))

		// SHARED HISTORY OPTIMIZATION:
		// If the linear commit already has the correct parent, we can just point the branch to it.
		// This ensures feature branches share the exact same commit objects as the linear history.
		if linearCommit.NumParents() > 0 && linearCommit.ParentHashes[0] == targetParentHash {
			logger.Info("      ⮑  %s", logger.StyleGrey.Render("[History matches, using linear commit directly]"))
			newCommitHash = linearH

			if !seepageCtx.Options.DryRun {
				refName := plumbing.NewBranchReferenceName(branchName)
				ref := plumbing.NewHashReference(refName, newCommitHash)
				_ = repo.Storer.SetReference(ref)
			}
			branchHeads[branchName] = newCommitHash
			continue
		}

		// CHERRY-PICK VIA TREE MERGE:
		// We want to combine the new parent's state with the bedrock's changes.
		targetCommit, err := repo.CommitObject(targetParentHash)
		if err != nil {
			continue
		}
		targetTree, err := targetCommit.Tree()
		if err != nil {
			continue
		}

		linearTree, err := linearCommit.Tree()
		if err != nil {
			continue
		}

		// Get all entries from target parent (unmanaged for this branch)
		entries := make(map[string]object.TreeEntry)
		if err := gitutil.GetAllEntriesRecursive(repo, targetTree, "", entries); err != nil {
			continue
		}

		// Get the rules for THIS bedrock
		managedPaths := seepageCtx.ResolvedRules[linearH.String()]

		// Step 1: Remove all entries from 'entries' that belong to managedPaths.
		// This handles deletions in the linear bedrock correctly.
		for path := range entries {
			for _, mp := range managedPaths {
				if models.BelongsToPath(path, mp) {
					delete(entries, path)
					break
				}
			}
		}

		// Step 2: Add all entries from 'linearTree' that belong to managedPaths.
		linearEntries := make(map[string]object.TreeEntry)
		_ = gitutil.GetAllEntriesRecursive(repo, linearTree, "", linearEntries)
		for path, entry := range linearEntries {
			for _, mp := range managedPaths {
				if models.BelongsToPath(path, mp) {
					entries[path] = entry
					break
				}
			}
		}

		newTreeHash, err := gitutil.BuildTree(repo, entries)
		if err != nil {
			logger.Error("      ⮑  Failed to build sediment tree: %v", err)
			continue
		}

		// ZERO-MUTATION / DAG PRUNING:
		refName := plumbing.NewBranchReferenceName(branchName)
		existingRef, err := repo.Reference(refName, true)
		if err == nil {
			existingCommit, err := repo.CommitObject(existingRef.Hash())
			if err == nil {
				if existingCommit.TreeHash == newTreeHash &&
					len(existingCommit.ParentHashes) > 0 && existingCommit.ParentHashes[0] == targetParentHash &&
					existingCommit.Message == linearCommit.Message {
					logger.Info("      ⮑  %s", logger.StyleGrey.Render("[Already sedimented]"))
					branchHeads[branchName] = existingRef.Hash()
					continue
				}
			}
		}

		// Create the sedimented commit
		newCommit := &object.Commit{
			Author:       linearCommit.Author,
			Committer:    linearCommit.Committer,
			Message:      linearCommit.Message,
			TreeHash:     newTreeHash,
			ParentHashes: []plumbing.Hash{targetParentHash},
		}

		obj := repo.Storer.NewEncodedObject()
		if err := newCommit.Encode(obj); err != nil {
			continue
		}
		newCommitHash, err = repo.Storer.SetEncodedObject(obj)
		if err != nil {
			continue
		}

		if !seepageCtx.Options.DryRun {
			ref := plumbing.NewHashReference(refName, newCommitHash)
			_ = repo.Storer.SetReference(ref)
		}

		branchHeads[branchName] = newCommitHash
		logger.Info("      ⮑  Success: %s", logger.ColorHash(newCommitHash.String()))
	}

	return nil
}
