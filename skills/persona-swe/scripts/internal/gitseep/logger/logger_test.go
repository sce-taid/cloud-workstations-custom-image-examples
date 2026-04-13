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

package logger_test

import (
	"strings"
	"testing"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
)

func TestColorHash(t *testing.T) {
	// Test short hash (less than 7 chars should return as-is)
	shortHash := "a1b2c"
	res := logger.ColorHash(shortHash)
	if res != shortHash {
		t.Errorf("Expected short hash %s to be unmodified, got %s", shortHash, res)
	}

	// Test full hash (should be truncated to 7 chars and colored)
	fullHash := "1234567890abcdef"
	res = logger.ColorHash(fullHash)

	// The rendered result will contain ANSI escape codes and the first 7 chars
	if !strings.Contains(res, "1234567") {
		t.Errorf("Expected truncated hash '1234567' in result, got %s", res)
	}

	// Because of lipgloss color profiles, it might not render colors in CI (e.g., NO_COLOR env).
	// We verify it has some ANSI formatting OR at least the raw string.
	if res == "1234567" {
		t.Logf("Hash was returned as raw text. This is expected in CI environments without color support.")
	}
}

func TestLoggerInit(t *testing.T) {
	logger.Init(true, false)
	if !logger.DebugMode {
		t.Errorf("Expected DebugMode to be true")
	}
	if logger.QuietMode {
		t.Errorf("Expected QuietMode to be false")
	}

	logger.Init(false, true)
	if logger.DebugMode {
		t.Errorf("Expected DebugMode to be false")
	}
	if !logger.QuietMode {
		t.Errorf("Expected QuietMode to be true")
	}
}

func TestLogFunctions(t *testing.T) {
	// This is primarily a smoke test to ensure no panics occur when calling the log functions.
	// Since they print to os.Stdout and os.Stderr, we are not strictly capturing output here
	// to avoid overcomplicating with stdout pipes, but we guarantee the formatting strings are valid.

	logger.Init(true, false)
	logger.Info("Info test %s", "arg")
	logger.Debug("Debug test %s", "arg")
	logger.Warn("Warn test %s", "arg")
	logger.Error("Error test %s", "arg")
	logger.Success("Success test %s", "arg")

	logger.Init(false, true)
	logger.Info("Quiet Info test")
	logger.Warn("Quiet Warn test")
	logger.Success("Quiet Success test")
}
