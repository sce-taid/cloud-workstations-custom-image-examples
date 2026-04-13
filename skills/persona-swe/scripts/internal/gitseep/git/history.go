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

// Package git provides helpers for interacting with the Git repository using go-git.
package git

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
)

// ResolveCommitByDate finds the first commit hash on or after a given date (YYYY-MM-DD) or an exact ISO date string in the current branch history.
func ResolveCommitByDate(repo *git.Repository, dateStr string) (string, error) {
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return "", err
	}

	var match *object.Commit

	// If it's a full ISO timestamp (e.g. "2026-04-14 17:36:44 +0000")
	if len(dateStr) > 10 {
		_ = cIter.ForEach(func(c *object.Commit) error {
			iso := c.Author.When.UTC().Format("2006-01-02 15:04:05 -0700")
			if strings.Contains(iso, dateStr) {
				match = c
				return fmt.Errorf("found") // Break early
			}
			return nil
		})
		if match != nil {
			return match.Hash.String(), nil
		}
	} else if len(dateStr) == 10 {
		// Fallback for YYYY-MM-DD: Find earliest commit on or after this day
		targetTime, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return "", err
		}
		var earliestMatch *object.Commit
		_ = cIter.ForEach(func(c *object.Commit) error {
			if c.Author.When.After(targetTime) || c.Author.When.Format("2006-01-02") == dateStr {
				if earliestMatch == nil || !c.Author.When.After(earliestMatch.Author.When) {
					earliestMatch = c
				}
			}
			return nil
		})
		if earliestMatch != nil {
			return earliestMatch.Hash.String(), nil
		}
	}

	return "", fmt.Errorf("no commits found for date %s in current branch history", dateStr)
}

// GetCommit returns a Git object.Commit from a hexadecimal hash string.
func GetCommit(repo *git.Repository, hashStr string) (*object.Commit, error) {
	hash := plumbing.NewHash(hashStr)
	return repo.CommitObject(hash)
}

// GetCurrentBranch returns the name of the branch currently checked out at HEAD.
func GetCurrentBranch(repo *git.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	return "HEAD", nil
}

// GetLinearHistory identifies the chronological range of strata commits between the earliest bedrock and HEAD.
func GetLinearHistory(repo *git.Repository, pathToBedrock map[string]string, baseCommit string) ([]string, string, string, string, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, "", "", "", err
	}
	origHead := head.Hash().String()

	currentBranch, _ := GetCurrentBranch(repo)

	// Sort bedrocks by commit time to find the oldest
	var bedrocks []string
	seen := make(map[string]bool)
	for _, h := range pathToBedrock {
		if !seen[h] {
			bedrocks = append(bedrocks, h)
			seen[h] = true
		}
	}

	var oldestBedrock *object.Commit
	for _, h := range bedrocks {
		c, err := GetCommit(repo, h)
		if err != nil {
			continue
		}
		if oldestBedrock == nil || c.Author.When.Before(oldestBedrock.Author.When) {
			oldestBedrock = c
		}
	}

	startPoint := oldestBedrock
	if baseCommit != "" {
		hash, err := repo.ResolveRevision(plumbing.Revision(baseCommit))
		if err != nil {
			// If not a revision, try it as a hash literal
			h := plumbing.NewHash(baseCommit)
			hash = &h
		}
		startPoint, err = repo.CommitObject(*hash)
		if err != nil {
			return nil, "", "", "", fmt.Errorf("invalid base commit or reference '%s': %v", baseCommit, err)
		}
	}

	var parentOfStrata string
	if startPoint != nil && startPoint.NumParents() > 0 {
		p, _ := startPoint.Parent(0)
		parentOfStrata = p.Hash.String()
	}

	// Get linear history from parent to original_head
	cIter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, "", "", "", err
	}

	var reverseStrata []string
	foundParent := false
	_ = cIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		if hash == parentOfStrata {
			foundParent = true
			return fmt.Errorf("found") // Stop
		}
		reverseStrata = append(reverseStrata, hash)
		return nil
	})

	if !foundParent && parentOfStrata != "" {
		// Log a warning or handle if needed, but don't just leave an empty branch
		logger.Debug("parent %s not found in history", parentOfStrata)
	}

	// Reverse the strata to be chronological
	var strata []string
	for i := len(reverseStrata) - 1; i >= 0; i-- {
		strata = append(strata, reverseStrata[i])
	}

	return strata, parentOfStrata, origHead, currentBranch, nil
}
