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
	"sort"
	"strings"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

// ValidateStratigraphyAndPredictConflicts runs cycle detection and conflict prediction to guarantee execution certitude.
func ValidateStratigraphyAndPredictConflicts(seepageCtx *models.SeepageContext, dr *models.DiscoveryResult) ([]string, error) {
	if len(seepageCtx.DateToBranch) == 0 {
		return []string{}, nil
	}

	logger.Info("\n--- Phase 0: Stratigraphy & Conflict Validation ---")

	branchNodes, orderedBranches, err := buildStratigraphyGraph(seepageCtx)
	if err != nil {
		return nil, err
	}

	renderStratigraphyTree(seepageCtx, branchNodes)

	if err := predictCherryPickConflicts(seepageCtx, dr, branchNodes, orderedBranches); err != nil {
		return nil, err
	}

	logger.Success("Stratigraphy validated successfully. No mathematical conflicts predicted.")
	return orderedBranches, nil
}

func buildStratigraphyGraph(seepageCtx *models.SeepageContext) (map[string]string, []string, error) {
	branchNodes := make(map[string]string)

	// Sort dates chronologically to establish implicit parents
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
		// Fallback to string comparison
		return dates[i] < dates[j]
	})

	for _, dateStr := range dates {
		branchName := seepageCtx.DateToBranch[dateStr]
		parentBranch := seepageCtx.BranchToParent[branchName]

		if parentBranch == "" {
			// Implicit Parent: the branch of the immediately preceding date
			idx := models.IndexOf(dates, dateStr)
			if idx > 0 {
				prevDate := dates[idx-1]
				parentBranch = seepageCtx.DateToBranch[prevDate]
			}
		}
		branchNodes[branchName] = parentBranch
	}

	orderedBranches := make([]string, 0)
	visited := make(map[string]bool)
	tempMark := make(map[string]bool)
	var path []string

	var visit func(string) error
	visit = func(n string) error {
		if tempMark[n] {
			// Construct full cycle description for the user.
			var cycle []string
			foundStart := false
			for _, p := range path {
				if p == n || foundStart {
					cycle = append(cycle, p)
					foundStart = true
				}
			}
			cycle = append(cycle, n)
			return fmt.Errorf("dependency cycle detected: %s", strings.Join(cycle, " -> "))
		}

		if !visited[n] {
			tempMark[n] = true
			path = append(path, n)

			p := branchNodes[n]
			if p != "" {
				if _, exists := branchNodes[p]; exists {
					if err := visit(p); err != nil {
						return err
					}
				}
			}

			path = path[:len(path)-1]
			delete(tempMark, n)
			visited[n] = true
			orderedBranches = append(orderedBranches, n)
		}
		return nil
	}

	// Sort keys for deterministic output
	var keys []string
	for k := range branchNodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, node := range keys {
		if !visited[node] {
			if err := visit(node); err != nil {
				return nil, nil, err
			}
		}
	}

	return branchNodes, orderedBranches, nil
}

func renderStratigraphyTree(seepageCtx *models.SeepageContext, branchNodes map[string]string) {
	var renderNode func(n string, prefix string, isLast bool)
	renderNode = func(n string, prefix string, isLast bool) {
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		if n == "BASE" {
			baseName := seepageCtx.BaseRef
			if baseName == "" {
				baseName = "base"
			}
			if seepageCtx.ParentOfStrata != "" {
				logger.Info("[%s] %s", logger.ColorHash(seepageCtx.ParentOfStrata), logger.StyleBold.Render(baseName))
			}
		}

		newPrefix := prefix
		if n != "BASE" {
			// Find the bedrock hash for this branch to display next to it
			var h string
			for d, b := range seepageCtx.DateToBranch {
				if b == n {
					h = seepageCtx.DateToHash[d]
					break
				}
			}

			if h != "" {
				logger.Info("[%s] %s%s%s", logger.ColorHash(h), prefix, connector, logger.StyleBold.Render(n))
			} else {
				logger.Info("          %s%s%s", prefix, connector, logger.StyleBold.Render(n))
			}

			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
		}

		var children []string
		for k, v := range branchNodes {
			if v == n || (n == "BASE" && (v == "" || !containsMapKey(branchNodes, v))) {
				children = append(children, k)
			}
		}
		sort.Strings(children)

		for i, child := range children {
			renderNode(child, newPrefix, i == len(children)-1)
		}
	}

	renderNode("BASE", "", true)
}

func predictCherryPickConflicts(
	seepageCtx *models.SeepageContext,
	dr *models.DiscoveryResult,
	branchNodes map[string]string,
	orderedBranches []string,
) error {
	for _, branchName := range orderedBranches {
		parentName := branchNodes[branchName]
		if parentName == "" || !containsMapKey(branchNodes, parentName) {
			continue
		}

		var s, pS string
		for d, b := range seepageCtx.DateToBranch {
			if b == branchName {
				s = seepageCtx.DateToHash[d]
			}
			if b == parentName {
				pS = seepageCtx.DateToHash[d]
			}
		}

		if s == "" || pS == "" {
			continue
		}

		sIdx := models.IndexOf(seepageCtx.Strata, s)
		psIdx := models.IndexOf(seepageCtx.Strata, pS)

		// Check range between parent bedrock and current bedrock
		startIdx := sIdx - 1
		if psIdx < startIdx {
			startIdx = psIdx
		}
		endIdx := sIdx - 1
		if psIdx > endIdx {
			endIdx = psIdx
		}

		var deltaCommits []string
		for i := startIdx + 1; i <= endIdx; i++ {
			if i >= 0 && i < len(seepageCtx.Strata) && i != sIdx {
				deltaCommits = append(deltaCommits, seepageCtx.Strata[i])
			}
		}

		for _, k := range deltaCommits {
			intersection := intersectSets(dr.Touched[s], dr.Touched[k])

			// FILTER: If a file is mapped to ANY bedrock, its state is isolated
			// by ReconstructHistory and will never cause a cherry-pick conflict.
			// It is a planned lithification/seepage.
			for f := range intersection {
				if h, _ := seepageCtx.Matcher.ResolveTarget(f); h != "" {
					delete(intersection, f)
				}
			}

			if len(intersection) > 0 {
				var files []string
				for f := range intersection {
					files = append(files, f)
				}
				sort.Strings(files)
				return fmt.Errorf(
					"conflict predicted: branch '%s' (bedrock %s) modifies files also touched by skipped stratum %s, files: %s - update your dependencies in .gitseep.yaml to resolve this conflict",
					branchName, logger.ColorHash(s), logger.ColorHash(k), strings.Join(files, ", "),
				)
			}
		}
	}
	return nil
}

func containsMapKey(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

func intersectSets(s1, s2 map[string]struct{}) map[string]struct{} {
	intersection := make(map[string]struct{})
	for k := range s1 {
		if _, ok := s2[k]; ok {
			intersection[k] = struct{}{}
		}
	}
	return intersection
}
