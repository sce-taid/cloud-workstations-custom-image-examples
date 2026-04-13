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

package git_test

import (
	"testing"
	"time"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/git"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
)

func TestGitHistorySuite(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()

	d0, _ := time.Parse("2006-01-02", "2026-04-14")
	d1, _ := time.Parse("2006-01-02", "2026-04-15")
	d2, _ := time.Parse("2006-01-02 15:04:05 -0700", "2026-04-16 10:00:00 +0000")

	h0, _ := testutil.CommitFile(repo, wt, "base.txt", "base", "base msg", d0)
	h1, _ := testutil.CommitFile(repo, wt, "file1.txt", "content1", "msg1", d1)
	h2, _ := testutil.CommitFile(repo, wt, "file2.txt", "content2", "msg2", d2)

	t.Run("Resolve Commit By Date", func(t *testing.T) {
		res1, err := git.ResolveCommitByDate(repo, "2026-04-15")
		if err != nil {
			t.Errorf("Failed to resolve YYYY-MM-DD: %v", err)
		}
		if res1 != h1 {
			t.Errorf("Expected %s, got %s", h1, res1)
		}

		res2, err := git.ResolveCommitByDate(repo, "2026-04-16 10:00:00 +0000")
		if err != nil {
			t.Errorf("Failed to resolve ISO date: %v", err)
		}
		if res2 != h2 {
			t.Errorf("Expected %s, got %s", h2, res2)
		}
	})

	t.Run("Get Linear History", func(t *testing.T) {
		paths := map[string]string{
			"file1.txt": h1,
		}

		strata, parent, origHead, branch, err := git.GetLinearHistory(repo, paths, "")
		if err != nil {
			t.Fatalf("GetLinearHistory failed: %v", err)
		}

		if branch != "master" { // Default branch for go-git mem repo
			t.Errorf("Expected master, got %s", branch)
		}
		if origHead != h2 {
			t.Errorf("Expected origHead %s, got %s", h2, origHead)
		}
		if parent != h0 {
			t.Errorf("Expected parent %s, got %s", h0, parent)
		}
		if len(strata) != 2 || strata[0] != h1 || strata[1] != h2 {
			t.Errorf("Expected strata [%s, %s], got %v", h1, h2, strata)
		}
	})
}
