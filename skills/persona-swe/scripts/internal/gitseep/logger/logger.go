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

package logger

import (
	"fmt"
	"hash/fnv"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// DebugMode enables verbose debug logging when true.
var DebugMode bool

// QuietMode suppresses all non-error output when true.
var QuietMode bool

var (
	// StyleBold renders text in bold.
	StyleBold   = lipgloss.NewStyle().Bold(true)
	// StyleGrey renders text in grey.
	StyleGrey   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	// StyleRed renders text in red.
	StyleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("31"))
	// StyleGreen renders text in green.
	StyleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("32"))
	// StyleYellow renders text in yellow.
	StyleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	// StyleBlack renders text in bold black.
	StyleBlack  = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Bold(true)
)

const (
	// IconBedrock represents a bedrock commit.
	IconBedrock   = "🪨"
	// IconPercolate represents a percolate down action.
	IconPercolate = "💧"
	// IconSeep represents a seep up action.
	IconSeep      = "🫧"
	// IconLithify represents a lithification action.
	IconLithify   = "💎"
	// IconBranch represents a branch creation/sync action.
	IconBranch    = "🏞️"
	// IconSuccess represents a successful action.
	IconSuccess   = "🏆"
	// IconWarn represents a warning.
	IconWarn      = "⚠️"
	// IconSelected represents a selected item.
	IconSelected  = "✅"
	// IconExcluded represents an excluded item.
	IconExcluded  = "❌"
	// IconSearch represents a search or discovery action.
	IconSearch    = "🔍"
)

// Init initializes the logger configuration.
func Init(debug, quiet bool) {
	DebugMode = debug
	QuietMode = quiet
}

// Info logs a message to stdout if QuietMode is false.
func Info(format string, a ...interface{}) {
	if !QuietMode {
		fmt.Fprintf(os.Stdout, format+"\n", a...)
	}
}

// Debug logs a verbose message to stdout if DebugMode is true.
func Debug(format string, a ...interface{}) {
	if DebugMode {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintf(os.Stdout, "%s\n", StyleGrey.Render("debug: "+msg))
	}
}

// Warn logs a warning message to stdout with an icon if QuietMode is false.
func Warn(format string, a ...interface{}) {
	if !QuietMode {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintf(os.Stdout, "%s %s %s\n", IconWarn, StyleBold.Render("warning:"), msg)
	}
}

// Error logs an error message to stderr.
func Error(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s\n", StyleRed.Render("error: "+msg))
}

// Success logs a success message to stdout with an icon if QuietMode is false.
func Success(format string, a ...interface{}) {
	if !QuietMode {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintf(os.Stdout, "%s %s\n", IconSuccess, msg)
	}
}

// ColorHash returns a truncated, deterministically colored, and bolded Git hash.
func ColorHash(h string) string {
	if len(h) < 7 {
		return h
	}
	short := h[:7]

	// High-contrast, vibrant colors from the xterm-256 palette.
	// Avoiding very dark or very light colors for readability.
	colors := []string{
		"1",   // Red
		"2",   // Green
		"3",   // Yellow
		"4",   // Blue
		"5",   // Magenta
		"6",   // Cyan
		"9",   // Bright Red
		"10",  // Bright Green
		"11",  // Bright Yellow
		"12",  // Bright Blue
		"13",  // Bright Magenta
		"14",  // Bright Cyan
		"202", // Orange
		"208", // Dark Orange
		"214", // Orange1
		"165", // Magenta3
		"135", // MediumPurple3
		"81",  // SteelBlue1
		"118", // Chartreuse1
		"226", // Yellow1
	}

	// Use FNV-1a hashing for deterministic and uniform color distribution.
	f := fnv.New32a()
	f.Write([]byte(h))
	hashVal := f.Sum32()

	c := colors[hashVal%uint32(len(colors))]
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c)).Render(short)
}
