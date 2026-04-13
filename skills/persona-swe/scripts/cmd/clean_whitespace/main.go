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

// Package main provides the entry point for the clean_whitespace utility.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/whitespace"
)

const (
	concurrencyLimit = 20
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: clean_whitespace <file_or_dir>...")
		os.Exit(0)
	}

	var paths []string
	for _, arg := range os.Args[1:] {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to stat %s: %v\n", arg, err)
			continue
		}

		if !info.IsDir() {
			paths = append(paths, arg)
			continue
		}

		err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				// Filter common source files to avoid binary noise.
				ext := strings.ToLower(filepath.Ext(path))
				switch ext {
				case ".go", ".py", ".ts", ".js", ".sh", ".bash", ".md", ".json", ".yaml", ".yml", ".bats", ".Dockerfile", ".conf", ".list":
					paths = append(paths, path)
				}
			}
			if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to walk directory %s: %v\n", arg, err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	modifiedCount := 0
	sem := make(chan struct{}, concurrencyLimit)

	for _, p := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			modified, err := whitespace.CleanFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to clean %s: %v\n", path, err)
				return
			}
			if modified {
				mu.Lock()
				fmt.Printf("Fixed: %s\n", path)
				modifiedCount++
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()

	if modifiedCount > 0 {
		fmt.Printf("\n⚠️  Cleaned trailing whitespace/EOF in %d files.\n", modifiedCount)
		os.Exit(1)
	}
}
