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

package git

import (
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
)

// GetStatusSummary checks the worktree and returns whether there are tracked changes and/or untracked files.
func GetStatusSummary(repo *git.Repository, experimental bool) (hasChanges bool, hasUntracked bool, err error) {
	if experimental {
		worktree, err := repo.Worktree()
		if err != nil {
			return false, false, err
		}
		status, err := worktree.Status()
		if err != nil {
			return false, false, err
		}

		for _, s := range status {
			if s.Worktree == git.Untracked || s.Staging == git.Untracked {
				hasUntracked = true
			} else if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
				hasChanges = true
			}
		}
		return hasChanges, hasUntracked, nil
	}

	// Default: use robust system git status
	wt, err := repo.Worktree()
	if err != nil {
		return false, false, err
	}
	root := wt.Filesystem.Root()

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return false, false, err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		if status == "??" {
			hasUntracked = true
		} else {
			hasChanges = true
		}
	}

	return hasChanges, hasUntracked, nil
}

// IsDirty returns true if there are any uncommitted changes (tracked or untracked).
func IsDirty(repo *git.Repository, experimental bool) (bool, error) {
	hasChanges, hasUntracked, err := GetStatusSummary(repo, experimental)
	return hasChanges || hasUntracked, err
}

// Amend performs a "git add -u" followed by a "git commit --amend --no-edit" to fold tracked changes into the last commit.
func Amend(repo *git.Repository) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	status, err := wt.Status()
	if err != nil {
		return err
	}

	// Add tracked but modified/deleted files (like git add -u)
	for path, s := range status {
		if s.Worktree != git.Unmodified && s.Worktree != git.Untracked {
			if _, err := wt.Add(path); err != nil {
				return err
			}
		}
	}

	head, err := repo.Head()
	if err != nil {
		return err
	}

	lastCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}

	_, err = wt.Commit(lastCommit.Message, &git.CommitOptions{
		Amend:  true,
		Author: &lastCommit.Author,
	})

	return err
}
