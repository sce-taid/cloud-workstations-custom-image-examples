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

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

func TestModelUpdateAndView(t *testing.T) {
	// Initialize a simple model with dummy data
	items := []item{
		{path: "file1.txt", bedrock: "h1", details: "🫧 ↑ h2"},
		{path: "file2.txt", bedrock: "h1", details: "💧 ↓ h3"},
	}

	seepageCtx := &models.SeepageContext{
		Strata: []string{"h1", "h2", "h3"},
	}

	dr := &models.DiscoveryResult{
		Sources: map[string]map[string]struct{}{
			"file1.txt": {"h2": {}},
			"file2.txt": {"h3": {}},
		},
	}

	m := model{
		seepageCtx: seepageCtx,
		dr:         dr,
		items:      items,
		excluded:   make(map[string]struct{}),
	}

	// Test default view
	view := m.View()
	if !strings.Contains(view, "file1.txt") || !strings.Contains(view, "file2.txt") {
		t.Errorf("View missing expected items")
	}
	if !strings.Contains(view, "🫧 ↑ h2") {
		t.Errorf("View missing compact details string")
	}

	// Test cursor movement
	msg := tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ := m.Update(msg)
	m = newModel.(model)
	if m.cursor != 1 {
		t.Errorf("Expected cursor to move to 1, got %d", m.cursor)
	}

	msg = tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	if m.cursor != 0 {
		t.Errorf("Expected cursor to move back to 0, got %d", m.cursor)
	}

	// Test selection toggle (exclude file1.txt)
	msg = tea.KeyMsg{Type: tea.KeySpace} // Space toggles exclusion
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	if _, ok := m.excluded["file1.txt"]; !ok {
		t.Errorf("Expected file1.txt to be excluded")
	}

	// Toggle again (include file1.txt)
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	if _, ok := m.excluded["file1.txt"]; ok {
		t.Errorf("Expected file1.txt to be re-included")
	}

	// Test quit/finalize
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	newModel, cmd := m.Update(msg)
	m = newModel.(model)
	if !m.quitting {
		t.Errorf("Expected 'c' to set quitting=true")
	}
	if cmd == nil {
		t.Errorf("Expected 'c' to return tea.Quit command")
	}

	// Test abort
	m.quitting = false
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	if !m.aborted || !m.quitting {
		t.Errorf("Expected 'q' to set aborted=true and quitting=true")
	}
}
