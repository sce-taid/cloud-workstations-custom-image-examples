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

// Package models defines shared data structures for GitSeep.
package models

// Options contains global configuration flags for a GitSeep execution.
type Options struct {
	TargetBranch      string
	BaseCommit        string
	AllFiles          bool
	AutoApprove       bool
	NoLithify         bool
	StageOnly         bool
	DryRun            bool
	CheckMode         bool
	Amend             bool
	Quiet             bool
	ExperimentalGoGit bool
}

// SeepageContext represents the resolved state of a GitSeep session.
type SeepageContext struct {
	RepoRoot       string
	OriginalHead   string
	CurrentBranch  string
	TargetBranch   string
	BaseRef        string
	ParentOfStrata string
	Strata         []string            // Chronological list of commit hashes
	ResolvedRules  map[string][]string // hash -> paths
	PathToBedrock  map[string]string   // path -> hash
	DateToBranch   map[string]string   // date -> branch name
	BranchToParent map[string]string   // branch name -> parent branch
	DateToHash     map[string]string   // date -> hash
	Options        Options
	Matcher        *RuleMatcher
}

// SeepageSummary accumulates telemetry and results for the final report.
type SeepageSummary struct {
	StrataProcessed    int
	BedrockFiles       int
	PercolateFiles     int
	SeepFiles          int
	LithifiedFiles     map[string][]string
	PercolatePaths     map[string]bool
	SeepPaths          map[string]bool
	SedimentedBranches map[string]bool
	ParityPassed       bool
}

// DiscoveryResult holds the mapping of files to their respective bedrock targets.
type DiscoveryResult struct {
	Schedule map[string]map[string][]string // bedrock_hash -> source_hash -> [files]
	Sources  map[string]map[string]struct{} // file_path -> set(source_hashes)
	Touched  map[string]map[string]struct{} // source_hash -> set(file_paths)
}

// MigrationItem represents a single file migration task.
type MigrationItem struct {
	Path    string
	Bedrock string
	Sources []string
}

// GetMigrationItems returns a flattened list of files that require migration (seep or percolate).
func (dr *DiscoveryResult) GetMigrationItems() []MigrationItem {
	pathData := make(map[string]map[string][]string)
	for bedrockH, sourcesMap := range dr.Schedule {
		for srcH, files := range sourcesMap {
			if srcH == bedrockH {
				continue
			}
			for _, f := range files {
				if pathData[f] == nil {
					pathData[f] = make(map[string][]string)
				}
				pathData[f][bedrockH] = append(pathData[f][bedrockH], srcH)
			}
		}
	}

	var items []MigrationItem
	for path, bedrocks := range pathData {
		for bedrockH, sources := range bedrocks {
			items = append(items, MigrationItem{
				Path:    path,
				Bedrock: bedrockH,
				Sources: sources,
			})
		}
	}
	return items
}
