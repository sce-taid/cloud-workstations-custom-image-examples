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

// Package ui provides the Terminal User Interface for GitSeep using Bubble Tea.
package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

// PrintPreflightBriefing displays the summary of strata and rules to the user.
func PrintPreflightBriefing(seepageCtx *models.SeepageContext, repo *git.Repository) {
	logger.Info("\n--- Pre-flight seepage briefing ---")
	logger.Info("Target branch: %s", seepageCtx.TargetBranch)
	logger.Info("Surface hash:  %s", logger.ColorHash(seepageCtx.OriginalHead))
	logger.Info("Source code bedrock assignments are listed under their commits.")
	logger.Info("Press [Q] at any prompt to abort without making changes.")
	logger.Info(strings.Repeat("-", 75))
	logger.Info(fmt.Sprintf("%-7s %-10s %-27s - %s", "Stratum", "[Commit]", "(Author Date)", "Message"))
	logger.Info(strings.Repeat("-", 75))

	for i := len(seepageCtx.Strata) - 1; i >= 0; i-- {
		h := seepageCtx.Strata[i]
		layerNum := len(seepageCtx.Strata) - i
		layerStr := fmt.Sprintf("%d/%d", layerNum, len(seepageCtx.Strata))

		c, err := repo.CommitObject(plumbing.NewHash(h))
		if err != nil {
			continue
		}
		date := c.Author.When.Format("2006-01-02 15:04:05 -0700")
		msg := strings.Split(c.Message, "\n")[0]

		logger.Info("%-7s %s (%s) - %s", layerStr, logger.ColorHash(h), date, msg)

		if paths, ok := seepageCtx.ResolvedRules[h]; ok {
			// Sort paths for consistency
			sort.Strings(paths)
			for _, p := range paths {
				logger.Info("            ⮑  %s", p)
			}
		}
	}
	logger.Info(strings.Repeat("-", 75))
}

// PerformReadOnlyReview executes Phase 1 of the stratigraphy review.
func PerformReadOnlyReview(seepageCtx *models.SeepageContext, dr *models.DiscoveryResult, repo *git.Repository) {
	logger.Info("\n--- Phase 1: Stratigraphy review (read-only) ---")

	for i := len(seepageCtx.Strata) - 1; i >= 0; i-- {
		commitHash := seepageCtx.Strata[i]
		actualIdx := i // 0-indexed chronologically

		c, _ := repo.CommitObject(plumbing.NewHash(commitHash))
		msg := strings.Split(c.Message, "\n")[0]
		layerStr := fmt.Sprintf("%d/%d", actualIdx+1, len(seepageCtx.Strata))

		logger.Info("\n%s %s - %s", layerStr, logger.ColorHash(commitHash), msg)

		var actions []string

		// Bedrock establishment
		if paths, ok := seepageCtx.ResolvedRules[commitHash]; ok {
			sort.Strings(paths)
			for _, ownedPath := range paths {
				sourcesForPath := make(map[string]struct{})

				for filePath, fileSources := range dr.Sources {
					if filePath == ownedPath || strings.HasPrefix(filePath, strings.TrimRight(ownedPath, "/")+"/") {
						bestMatch := ""
						for rulePath := range seepageCtx.PathToBedrock {
							if filePath == rulePath || strings.HasPrefix(filePath, strings.TrimRight(rulePath, "/")+"/") {
								if bestMatch == "" || len(rulePath) > len(bestMatch) {
									bestMatch = rulePath
								}
							}
						}
						if bestMatch == ownedPath {
							for src := range fileSources {
								sourcesForPath[src] = struct{}{}
							}
						}
					}
				}

				if len(sourcesForPath) == 0 {
					if seepageCtx.Options.AllFiles {
						actions = append(actions, fmt.Sprintf("- %s %s (Bedrock - No changes found in range)", logger.StyleBlack.Render(ownedPath), logger.IconBedrock))
					}
					continue
				}

				// Foreign sources
				foreignSources := make(map[string]struct{})
				for src := range sourcesForPath {
					if src != commitHash {
						foreignSources[src] = struct{}{}
					}
				}

				if len(foreignSources) > 0 {
					percCount := 0
					seepCount := 0
					for src := range foreignSources {
						srcIdx := models.IndexOf(seepageCtx.Strata, src)
						if srcIdx > actualIdx {
							percCount++
						} else {
							seepCount++
						}
					}

					var details []string
					if seepCount > 0 {
						details = append(details, fmt.Sprintf("↑ Seep Up from %d older strata", seepCount))
					}
					if percCount > 0 {
						details = append(details, fmt.Sprintf("↓ Percolate Down from %d newer strata", percCount))
					}

					lithWarn := ""
					if len(sourcesForPath) > 1 {
						lithWarn = fmt.Sprintf(" %s%s Lithify%s", "\033[90m", logger.IconLithify, "\033[0m") // Grey
					}
					actions = append(actions, fmt.Sprintf("- %s %s (Establishing via %s)%s", logger.StyleBlack.Render(ownedPath), logger.IconBedrock, strings.Join(details, ", "), lithWarn))
				} else if seepageCtx.Options.AllFiles {
					actions = append(actions, fmt.Sprintf("- %s %s (Established natively)", logger.StyleBlack.Render(ownedPath), logger.IconBedrock))
				}

				// Also list individual native files
				if seepageCtx.Options.AllFiles {
					if hashMap, ok := dr.Schedule[commitHash]; ok {
						if nativeFiles, ok := hashMap[commitHash]; ok {
							sort.Strings(nativeFiles)
							for _, f := range nativeFiles {
								if f == ownedPath || strings.HasPrefix(f, strings.TrimRight(ownedPath, "/")+"/") {
									actions = append(actions, fmt.Sprintf("- %s %s (Established natively)", logger.StyleBlack.Render(f), logger.IconBedrock))
								}
							}
						}
					}
				}
			}
		}

		// Foreign Seepage
		for bedrockH, sourcesMap := range dr.Schedule {
			if bedrockH == commitHash {
				continue
			}
			if files, ok := sourcesMap[commitHash]; ok {
				direction := "Seep Up"
				icon := logger.IconSeep
				if models.IndexOf(seepageCtx.Strata, commitHash) > models.IndexOf(seepageCtx.Strata, bedrockH) {
					direction = "Percolate Down"
					icon = logger.IconPercolate
				}

				sort.Strings(files)
				for _, f := range files {
					actions = append(actions, fmt.Sprintf("- %s %s (%s to Bedrock %s)", logger.StyleBlack.Render(f), icon, direction, logger.ColorHash(bedrockH)))
				}
			}
		}

		sort.Strings(actions)
		for _, a := range actions {
			logger.Info(a)
		}
	}
}

// PerformSelectionPhase executes Phase 2 interactive UI for selective file exclusion.
func PerformSelectionPhase(seepageCtx *models.SeepageContext, dr *models.DiscoveryResult, repo *git.Repository) error {
	if seepageCtx.Options.AutoApprove {
		return nil
	}

	migrationItems := dr.GetMigrationItems()
	var items []item
	for _, mi := range migrationItems {
		sort.Slice(mi.Sources, func(i, j int) bool {
			return models.IndexOf(seepageCtx.Strata, mi.Sources[i]) < models.IndexOf(seepageCtx.Strata, mi.Sources[j])
		})

		var details []string
		for _, srcH := range mi.Sources {
			srcIdx := models.IndexOf(seepageCtx.Strata, srcH)
			bedIdx := models.IndexOf(seepageCtx.Strata, mi.Bedrock)

			icon := logger.IconSeep
			dir := "↑"
			if srcIdx > bedIdx {
				icon = logger.IconPercolate
				dir = "↓"
			}
			details = append(details, fmt.Sprintf("%s %s %s", icon, dir, logger.ColorHash(srcH)))
		}

		items = append(items, item{
			path:    mi.Path,
			bedrock: mi.Bedrock,
			details: strings.Join(details, ", "),
		})
	}

	if len(items) == 0 {
		return nil
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].path < items[j].path
	})

	m := model{
		seepageCtx: seepageCtx,
		dr:         dr,
		repo:       repo,
		items:      items,
		excluded:   make(map[string]struct{}),
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running UI: %w", err)
	}

	if finalModel.(model).aborted {
		logger.Info("\n%s Seepage process cancelled. Your repository remains untouched.", logger.StyleGrey.Render("[👋 Goodbye]"))
		return fmt.Errorf("aborted")
	}

	// Update schedule based on exclusions
	finalExcluded := finalModel.(model).excluded
	newSchedule := make(map[string]map[string][]string)

	for bedrockH, sourcesMap := range dr.Schedule {
		for srcH, files := range sourcesMap {
			var filtered []string
			for _, f := range files {
				if _, ok := finalExcluded[f]; !ok {
					filtered = append(filtered, f)
				}
			}
			if len(filtered) > 0 {
				if newSchedule[bedrockH] == nil {
					newSchedule[bedrockH] = make(map[string][]string)
				}
				newSchedule[bedrockH][srcH] = filtered
			}
		}
	}
	dr.Schedule = newSchedule
	return nil
}

type item struct {
	path    string
	bedrock string
	details string
}

type model struct {
	seepageCtx *models.SeepageContext
	dr         *models.DiscoveryResult
	repo       *git.Repository
	items      []item
	cursor     int
	excluded   map[string]struct{}
	quitting   bool
	aborted    bool
	width      int
	height     int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) pageSize() int {
	// Header is ~7 lines, footer is ~2 lines, each item is 2 lines
	if m.height <= 10 {
		return 5
	}
	return (m.height - 10) / 2
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit
		case "c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "pgup", "ctrl+b":
			m.cursor -= m.pageSize()
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown", "ctrl+f":
			m.cursor += m.pageSize()
			if m.cursor >= len(m.items) {
				m.cursor = len(m.items) - 1
			}
		case " ", "enter", "x":
			path := m.items[m.cursor].path
			if _, ok := m.excluded[path]; ok {
				delete(m.excluded, path)
			} else {
				m.excluded[path] = struct{}{}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 80
	}

	s := "\n--- Phase 2: Selective exclusion ---\n"
	s += "Use [UP/DOWN] to navigate, [PGUP/PGDOWN] to page, [SPACE/ENTER/X] to toggle selection.\n"
	s += fmt.Sprintf("Legend: [%s] = Selected for migration, [%s] = Excluded from migration.\n", logger.IconSelected, logger.IconExcluded)
	s += "Press [C] to finalize selection and continue.\n"
	s += "Press [Q] or [Ctrl+C] to abort any time without action.\n"
	s += strings.Repeat("-", w) + "\n"

	windowSize := m.pageSize()
	if windowSize < 1 {
		windowSize = 1
	}

	start := m.cursor - (windowSize / 2)
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := start; i < end; i++ {
		it := m.items[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "\033[7m>\033[0m "
		}

		icon := logger.IconSelected
		if _, ok := m.excluded[it.path]; ok {
			icon = logger.IconExcluded
		}

		lithLabel := ""
		if len(m.dr.Sources[it.path]) > 1 {
			lithLabel = fmt.Sprintf(" %s%s Lithify%s", "\033[90m", logger.IconLithify, "\033[0m")
		}

		s += fmt.Sprintf("%s[%s] %s%s\n", prefix, icon, logger.StyleBlack.Render(it.path), lithLabel)
		s += fmt.Sprintf("    ⮑  to Bedrock %s: %s\n", logger.ColorHash(it.bedrock), it.details)
	}
	s += strings.Repeat("-", w) + "\n"
	selectedCount := len(m.items) - len(m.excluded)
	s += fmt.Sprintf("Item %d of %d (%d/%d selected for migration)\n", m.cursor+1, len(m.items), selectedCount, len(m.items))

	return s
}
