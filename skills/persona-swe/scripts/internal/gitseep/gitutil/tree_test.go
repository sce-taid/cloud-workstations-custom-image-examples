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

package gitutil_test

import (
	"testing"
	"time"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/gitutil"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/testutil"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGetAllEntriesAndBuildTree(t *testing.T) {
	repo, wt := testutil.SetupMemRepo()
	d0 := time.Now()

	_, _ = testutil.CommitFile(repo, wt, "src/main.go", "package main", "init", d0)
	testutil.CommitFile(repo, wt, "README.md", "hello", "doc", d0.Add(time.Hour))
	
	head, _ := repo.Head()
	headCommit, _ := repo.CommitObject(head.Hash())
	headTree, _ := headCommit.Tree()

	entries, err := gitutil.GetAllEntries(repo, headTree)
	if err != nil {
		t.Fatalf("GetAllEntries failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 files, got %d", len(entries))
	}

	if _, ok := entries["src/main.go"]; !ok {
		t.Errorf("Missing src/main.go")
	}

	if _, ok := entries["README.md"]; !ok {
		t.Errorf("Missing README.md")
	}

	// Rebuild tree from the entries
	newTreeHash, err := gitutil.BuildTree(repo, entries)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if newTreeHash.IsZero() {
		t.Fatalf("BuildTree returned zero hash")
	}

	if newTreeHash != headCommit.TreeHash {
		t.Errorf("Rebuilt tree hash %v does not match original tree hash %v", newTreeHash, headCommit.TreeHash)
	}

	// Test building empty tree
	emptyEntries := make(map[string]object.TreeEntry)
	emptyTreeHash, err := gitutil.BuildTree(repo, emptyEntries)
	if err != nil {
		t.Fatalf("BuildTree empty failed: %v", err)
	}
	if emptyTreeHash.IsZero() {
		t.Errorf("Expected valid hash for empty tree, got zero")
	}
}
