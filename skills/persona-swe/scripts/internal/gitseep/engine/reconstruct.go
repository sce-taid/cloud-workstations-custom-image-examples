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
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/gitutil"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

// ReconstructHistory rebuilds the Git history purely in-memory by projecting the final state of owned paths into historical strata.
func ReconstructHistory(seepageCtx *models.SeepageContext, dr *models.DiscoveryResult, repo *git.Repository) (map[string]string, error) {
	logger.Info("\n--- Phase 3: Execution (history reconstruction) ---")

	summary := &models.SeepageSummary{
		LithifiedFiles:     make(map[string][]string),
		PercolatePaths:     make(map[string]bool),
		SeepPaths:          make(map[string]bool),
		SedimentedBranches: make(map[string]bool),
	}

	linearCommits := make(map[string]string)

	currentParentHash := plumbing.NewHash(seepageCtx.ParentOfStrata)
	origHeadHash := plumbing.NewHash(seepageCtx.OriginalHead)
	origHeadCommit, err := repo.CommitObject(origHeadHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get original head commit: %w", err)
	}
	origHeadTree, err := origHeadCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree for original head: %w", err)
	}
	headFiles, err := gitutil.GetAllEntries(repo, origHeadTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get files for original head: %w", err)
	}

	var ownedPaths []string
	for p := range seepageCtx.PathToBedrock {
		ownedPaths = append(ownedPaths, p)
	}

	// Build a map of files selected for migration in the UI
	selectedFiles := make(map[string]bool)
	for _, sourcesMap := range dr.Schedule {
		for _, files := range sourcesMap {
			for _, f := range files {
				selectedFiles[f] = true
			}
		}
	}

	for i, commitHashStr := range seepageCtx.Strata {
		cHash := plumbing.NewHash(commitHashStr)
		origCommit, err := repo.CommitObject(cHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get original commit %s: %w", commitHashStr, err)
		}

		logger.Info("\n%d/%d %s - %s", i+1, len(seepageCtx.Strata), logger.ColorHash(commitHashStr), strings.Split(origCommit.Message, "\n")[0])

		origTree, err := origCommit.Tree()
		if err != nil {
			return nil, fmt.Errorf("failed to get tree for original commit %s: %w", commitHashStr, err)
		}
		origFiles, err := gitutil.GetAllEntries(repo, origTree)
		if err != nil {
			return nil, fmt.Errorf("failed to get files for original commit %s: %w", commitHashStr, err)
		}

		parentCommit, err := repo.CommitObject(currentParentHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent commit %s: %w", currentParentHash.String(), err)
		}
		parentTree, err := parentCommit.Tree()
		if err != nil {
			return nil, fmt.Errorf("failed to get tree for parent commit %s: %w", currentParentHash.String(), err)
		}
		parentFiles, err := gitutil.GetAllEntries(repo, parentTree)
		if err != nil {
			return nil, fmt.Errorf("failed to get files for parent commit %s: %w", currentParentHash.String(), err)
		}

		// 1. Calculate Target State
		allPaths := make(map[string]struct{})
		for p := range origFiles {
			allPaths[p] = struct{}{}
		}
		for p := range headFiles {
			allPaths[p] = struct{}{}
		}
		for p := range parentFiles {
			allPaths[p] = struct{}{}
		}

		cFiles := make(map[string]object.TreeEntry)
		var filesCaptured []string

		for p := range allPaths {
			bestMatch := ""
			_, bestMatch = seepageCtx.Matcher.ResolveTarget(p)

			// Logic:
			// 1. If it is Managed (owned AND selected in UI):
			if bestMatch != "" && selectedFiles[p] {
				bedrockHash := seepageCtx.PathToBedrock[bestMatch]
				if commitHashStr == bedrockHash {
					// This is the TARGET bedrock for this file. Pull from HEAD.
					if e, ok := headFiles[p]; ok {
						cFiles[p] = e
						if !containsString(filesCaptured, bestMatch) {
							filesCaptured = append(filesCaptured, bestMatch)
							logger.Info("  [%s] Capturing sediment for '%s'", logger.IconBedrock, logger.StyleBlack.Render(bestMatch))
						}
						summary.BedrockFiles++
					}
				} else {
					// This file belongs to a DIFFERENT bedrock.
					// Carry forward whatever state we've reconstructed so far.
					if e, ok := parentFiles[p]; ok {
						cFiles[p] = e
					}
				}
			} else {
				// 2. Unmanaged files or UI-excluded files:
				// They should follow their original history.
				if e, ok := origFiles[p]; ok {
					cFiles[p] = e
				} else if i == len(seepageCtx.Strata)-1 {
					// SURFACE INJECTION: If it's the FINAL commit in the stack,
					// and the file exists at the Surface (HEAD) but wasn't in history,
					// include it. This preserves .gitseep.yaml and new work.
					if e, ok := headFiles[p]; ok {
						cFiles[p] = e
					}
				}
			}
		}

		// 2. Build the new tree
		newTreeHash, err := gitutil.BuildTree(repo, cFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to build tree for %s: %w", commitHashStr, err)
		}

		// 3. Zero-Mutation Optimization
		if newTreeHash == origCommit.TreeHash && len(origCommit.ParentHashes) > 0 && currentParentHash == origCommit.ParentHashes[0] {
			currentParentHash = origCommit.Hash
			linearCommits[commitHashStr] = origCommit.Hash.String()
			summary.StrataProcessed++
			continue
		}

		if seepageCtx.Options.CheckMode {
			return nil, fmt.Errorf("please run 'gitseep' to bring the stratigraphy in order (commit %s requires modification)", logger.ColorHash(commitHashStr))
		}

		// 4. Create the new commit
		newCommit := object.Commit{
			Author:       origCommit.Author,
			Committer:    origCommit.Committer,
			Message:      origCommit.Message,
			TreeHash:     newTreeHash,
			ParentHashes: []plumbing.Hash{currentParentHash},
		}

		obj := repo.Storer.NewEncodedObject()
		if err := newCommit.Encode(obj); err != nil {
			return nil, fmt.Errorf("failed to encode new commit: %w", err)
		}
		newCommitHash, err := repo.Storer.SetEncodedObject(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to set encoded object for new commit: %w", err)
		}

		currentParentHash = newCommitHash
		linearCommits[commitHashStr] = newCommitHash.String()
		summary.StrataProcessed++
	}

	// Update Target Branch
	if !seepageCtx.Options.DryRun {
		refName := plumbing.NewBranchReferenceName(seepageCtx.TargetBranch)
		if seepageCtx.Options.StageOnly {
			refName = plumbing.NewBranchReferenceName("gitseep-staged")
			logger.Info("\n--- Stage-only mode ---")
			logger.Info("Refactored history prepared on: gitseep-staged")
		}

		ref := plumbing.NewHashReference(refName, currentParentHash)
		if err := repo.Storer.SetReference(ref); err != nil {
			return nil, fmt.Errorf("failed to update branch reference %s: %w", refName.String(), err)
		}

		if !seepageCtx.Options.StageOnly {
			// Update HEAD if we are currently ON the target branch
			head, _ := repo.Head()
			if head.Name() == refName {
				if seepageCtx.Options.ExperimentalGoGit {
					w, err := repo.Worktree()
					if err == nil {
						// SAFE CHECKOUT: Force set to FALSE to protect untracked files.
						err = w.Checkout(&git.CheckoutOptions{
							Branch: refName,
							Force:  false,
						})
						if err != nil {
							logger.Warn("Failed to checkout updated branch to worktree: %v", err)

							// Inspect worktree to list problematic files
							status, sErr := w.Status()
							if sErr == nil && !status.IsClean() {
								var dirty []string
								for path, s := range status {
									if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
										dirty = append(dirty, path)
									}
								}
								if len(dirty) > 0 {
									sort.Strings(dirty)
									logger.Info("The following files have local changes and would be overwritten:")
									for _, f := range dirty {
										logger.Info("  %s", f)
									}
								}
							}
							logger.Warn("Please commit, stage, or stash your changes before running gitseep.")
						}
					}
				} else {
					// Use standard system Git to update the worktree robustly
					cmd := exec.Command("git", "reset", "--hard", currentParentHash.String())
					cmd.Dir = seepageCtx.RepoRoot
					// If RepoRoot is empty (not explicitly set in some tests/contexts), default to current directory
					if cmd.Dir == "" {
						cmd.Dir = "."
					}
					if err := cmd.Run(); err != nil {
						logger.Warn("Failed to synchronize worktree using system git: %v", err)
						logger.Warn("Please run 'git reset --hard' manually to complete the update.")
					}
				}
			}
			logger.Info("\nBranch '%s' has been updated.", seepageCtx.TargetBranch)
		}
	}

	return linearCommits, nil
}

// Tree Building Helpers

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
