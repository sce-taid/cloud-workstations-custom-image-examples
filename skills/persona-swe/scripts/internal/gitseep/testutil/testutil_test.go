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

package testutil_test

import (
	"testing"
	"time"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
)

func TestSetupMemRepo(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()

	if repo == nil {
		t.Fatalf("Expected repository to be initialized")
	}

	if wt == nil {
		t.Fatalf("Expected worktree to be initialized")
	}

	// Commit a file to test utility functionality
	date := time.Now()
	hash, err := testutil.CommitFile(repo, wt, "test.txt", "content", "msg", date)
	if err != nil {
		t.Fatalf("CommitFile failed: %v", err)
	}

	if hash == "" {
		t.Fatalf("Expected a valid commit hash, got empty string")
	}

	// Remove a file to test utility functionality
	_, err = testutil.RemoveFile(repo, wt, "test.txt", "remove msg", date.Add(time.Hour))
	if err != nil {
		t.Fatalf("RemoveFile failed: %v", err)
	}
}
