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

package models

// RuleMatcher handles the mapping of file paths to architectural bedrock commits.
type RuleMatcher struct {
	PathToBedrock map[string]string
	OwnedPaths    []string
}

// NewRuleMatcher initializes a matcher with the provided path-to-bedrock mapping.
func NewRuleMatcher(pathToBedrock map[string]string) *RuleMatcher {
	owned := make([]string, 0, len(pathToBedrock))
	for p := range pathToBedrock {
		owned = append(owned, p)
	}
	return &RuleMatcher{
		PathToBedrock: pathToBedrock,
		OwnedPaths:    owned,
	}
}

// ResolveTarget returns the bedrock hash and the matching rule path for a given file.
// It follows the "longest match wins" principle.
func (m *RuleMatcher) ResolveTarget(filePath string) (bedrockHash string, rulePath string) {
	longestMatch := -1
	for _, rp := range m.OwnedPaths {
		if BelongsToPath(filePath, rp) {
			if len(rp) > longestMatch {
				longestMatch = len(rp)
				rulePath = rp
				bedrockHash = m.PathToBedrock[rp]
			}
		}
	}
	return bedrockHash, rulePath
}

// BelongsToPath checks if a file path is governed by a rule path (prefix matching).
func BelongsToPath(filePath, rulePath string) bool {
	if filePath == rulePath {
		return true
	}
	// Handle directory matching: /src matches /src/main.go
	if len(rulePath) > 0 && rulePath[len(rulePath)-1] != '/' {
		return filePath == rulePath || (len(filePath) > len(rulePath) && filePath[len(rulePath)] == '/' && filePath[:len(rulePath)] == rulePath)
	}
	return len(filePath) >= len(rulePath) && filePath[:len(rulePath)] == rulePath
}
