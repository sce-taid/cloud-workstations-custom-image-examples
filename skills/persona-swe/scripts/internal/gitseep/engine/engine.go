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

// Package engine orchestrates the core GitSeep logic including discovery,
// reconstruction, and sedimentation phases.
package engine

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/config"
	gitseepGit "github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/git"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/ui"
)

// Check performs a read-only validation of the repository stratigraphy.
func Check(cfg *config.GitSeepConfig, opts models.Options) error {
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	opts.DryRun = true
	opts.CheckMode = true

	seepageCtx, err := NewContext(repo, cfg, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize context: %w", err)
	}

	dr, err := Discover(seepageCtx, repo)
	if err != nil {
		return fmt.Errorf("discovery phase failed: %w", err)
	}

	orderedBranches, err := ValidateStratigraphyAndPredictConflicts(seepageCtx, dr)
	if err != nil {
		return fmt.Errorf("stratigraphy validation failed: %w", err)
	}

	linearCommits, err := ReconstructHistory(seepageCtx, dr, repo)
	if err != nil {
		return fmt.Errorf("history reconstruction failed: %w", err)
	}

	if err := PerformSedimentation(seepageCtx, repo, orderedBranches, linearCommits); err != nil {
		return fmt.Errorf("sedimentation phase failed: %w", err)
	}

	logger.Success("GitSeep check passed. Stratigraphy is perfectly sedimented.")
	return nil
}

// Run executes the full GitSeep pipeline.
func Run(cfg *config.GitSeepConfig, opts models.Options) error {
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	if err := VerifyWorktreeIsClean(repo, opts); err != nil {
		return err
	}

	seepageCtx, err := NewContext(repo, cfg, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize context: %w", err)
	}

	logger.Debug("Repository opened and mapped successfully")

	dr, err := Discover(seepageCtx, repo)
	if err != nil {
		return fmt.Errorf("discovery phase failed: %w", err)
	}

	orderedBranches, err := ValidateStratigraphyAndPredictConflicts(seepageCtx, dr)
	if err != nil {
		return fmt.Errorf("stratigraphy validation failed: %w", err)
	}

	// Interactive UI Phases
	ui.PrintPreflightBriefing(seepageCtx, repo)
	ui.PerformReadOnlyReview(seepageCtx, dr, repo)
	if err := ui.PerformSelectionPhase(seepageCtx, dr, repo); err != nil {
		if err.Error() == "aborted" {
			return nil
		}
		return err
	}

	// Count selected files
	selectedCount := 0
	for _, sourcesMap := range dr.Schedule {
		for srcH, files := range sourcesMap {
			// Skip bedrock targets where no migration is happening (srcH == bedrockH was already filtered for UI)
			// but dr.Schedule still contains them. Phase 2 filtering only affects the interactive list.
			// However, PerformSelectionPhase updates dr.Schedule based on UI results.
			// Actually, let's just count everything remaining in dr.Schedule.
			// If PerformSelectionPhase only listed seep/percolate, then dr.Schedule
			// will only contain those seep/percolate entries (plus original bedrock entries which we might want to skip counting).

			// Wait, dr.Schedule is updated by PerformSelectionPhase.
			// If the user excluded everything from the list, dr.Schedule might still have bedrock entries.
			// Let's count ONLY seep/percolate.
			bedrockH := ""
			for bH, sMap := range dr.Schedule {
				if sMap[srcH] != nil {
					bedrockH = bH
					break
				}
			}

			if srcH != bedrockH {
				selectedCount += len(files)
			}
		}
	}

	if selectedCount == 0 {
		logger.Success("No files selected for migration. History is already perfectly sedimented.")
		return nil
	}

	linearCommits, err := ReconstructHistory(seepageCtx, dr, repo)
	if err != nil {
		return fmt.Errorf("history reconstruction failed: %w", err)
	}

	if err := PerformSedimentation(seepageCtx, repo, orderedBranches, linearCommits); err != nil {
		return fmt.Errorf("sedimentation phase failed: %w", err)
	}

	logger.Success("GitSeep pipeline completed successfully.")
	return nil
}

// VerifyWorktreeIsClean ensures the workspace is in a safe state for orchestration.
func VerifyWorktreeIsClean(repo *git.Repository, opts models.Options) error {
	hasChanges, hasUntracked, err := gitseepGit.GetStatusSummary(repo, opts.ExperimentalGoGit)
	if err != nil {
		return fmt.Errorf("failed to check worktree status: %w", err)
	}

	if hasChanges {
		if opts.Amend {
			logger.Info("Amending tracked changes into last commit...")
			if err := gitseepGit.Amend(repo); err != nil {
				return fmt.Errorf("failed to amend changes: %w", err)
			}
		} else {
			return fmt.Errorf("working tree has uncommitted changes. Please commit, stash, or use --amend before running gitseep")
		}
	}

	if hasUntracked {
		logger.Info("%s Note: You have untracked files. If you want them included in your history, use 'git add' first.", logger.StyleGrey.Render("[ℹ️]"))
	}

	return nil
}

// NewContext resolves bedrock commits and maps the linear history into a SeepageContext.
func NewContext(repo *git.Repository, cfg *config.GitSeepConfig, opts models.Options) (*models.SeepageContext, error) {
	seepageCtx := &models.SeepageContext{
		BaseRef:        cfg.Global.BaseRef,
		ResolvedRules:  make(map[string][]string),
		PathToBedrock:  make(map[string]string),
		DateToBranch:   make(map[string]string),
		BranchToParent: make(map[string]string),
		DateToHash:     make(map[string]string),
		Options:        opts,
	}

	if err := resolveBedrockCommits(repo, cfg, seepageCtx); err != nil {
		return nil, err
	}

	strata, parent, origHead, currentBranch, err := gitseepGit.GetLinearHistory(repo, seepageCtx.PathToBedrock, opts.BaseCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate linear history: %w", err)
	}

	seepageCtx.Strata = strata
	seepageCtx.ParentOfStrata = parent
	seepageCtx.OriginalHead = origHead
	seepageCtx.CurrentBranch = currentBranch

	if seepageCtx.TargetBranch == "" {
		seepageCtx.TargetBranch = currentBranch
	}

	seepageCtx.Matcher = models.NewRuleMatcher(seepageCtx.PathToBedrock)

	return seepageCtx, nil
}

// resolveBedrockCommits matches declarative rules to actual Git objects based on Author Date.
func resolveBedrockCommits(repo *git.Repository, cfg *config.GitSeepConfig, seepageCtx *models.SeepageContext) error {
	logger.Debug("Resolving Bedrock commits from rules...")
	for _, rule := range cfg.Rules {
		hash, err := gitseepGit.ResolveCommitByDate(repo, rule.Date)
		if err != nil {
			return fmt.Errorf("failed to resolve bedrock date %s: %w", rule.Date, err)
		}

		branchName := rule.Branch
		if branchName == "" {
			branchName = "sediment/" + rule.Date
		}

		seepageCtx.DateToBranch[rule.Date] = branchName
		seepageCtx.BranchToParent[branchName] = rule.Parent
		seepageCtx.DateToHash[rule.Date] = hash
		seepageCtx.ResolvedRules[hash] = rule.Paths

		for _, p := range rule.Paths {
			seepageCtx.PathToBedrock[p] = hash
		}
	}
	return nil
}

// engine helpers

func parseRuleDate(dateStr string) time.Time {
	// Try parsing full ISO format
	if len(dateStr) > 10 {
		t, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err == nil {
			return t
		}
	}
	// Try YYYY-MM-DD
	t, err := time.Parse("2006-01-02", dateStr)
	if err == nil {
		return t
	}
	return time.Time{}
}
