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

package testutil

import (
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

// SetupMemRepo creates a fast, in-memory Git repository for testing.
func SetupMemRepo() (*git.Repository, *git.Worktree) {
	storer := memory.NewStorage()
	fs := memfs.New()
	repo, _ := git.Init(storer, fs)
	wt, _ := repo.Worktree()
	return repo, wt
}

// CommitFile creates or updates a file and commits it to the repo.
func CommitFile(repo *git.Repository, wt *git.Worktree, path, content, msg string, authorDate time.Time) (string, error) {
	f, err := wt.Filesystem.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := f.Write([]byte(content)); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	_, err = wt.Add(path)
	if err != nil {
		return "", err
	}

	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  authorDate,
		},
	})
	if err != nil {
		if err.Error() == "cannot create empty commit: clean working tree" {
			// If nothing to commit (e.g. file already gone), just return empty
			return "", nil
		}
		return "", err
	}

	return hash.String(), nil
}

// RemoveFile deletes a file and commits the removal.
func RemoveFile(repo *git.Repository, wt *git.Worktree, path, msg string, authorDate time.Time) (string, error) {
	// Ensure the index is up to date before removing
	if _, err := wt.Add(path); err != nil {
		return "", err
	}

	// In memfs/go-git, we need to ensure the file is removed from the worktree index
	if _, err := wt.Remove(path); err != nil {
		return "", err
	}

	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  authorDate,
		},
	})
	if err != nil {
		if err.Error() == "cannot create empty commit: clean working tree" {
			// If nothing to commit (e.g. file already gone), just return empty
			return "", nil
		}
		return "", err
	}

	return hash.String(), nil
}
