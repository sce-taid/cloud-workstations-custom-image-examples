package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// cleanFile removes trailing whitespace and ensures exactly one newline at EOF.
func cleanFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	if len(data) == 0 {
		return false, nil
	}

	// Skip binary files (naive check for null byte)
	if bytes.IndexByte(data, 0) != -1 {
		return false, nil
	}

	// Split by newline
	lines := bytes.Split(data, []byte("\n"))
	var cleanedLines [][]byte

	// 1. Trim trailing spaces/tabs from each line
	for _, line := range lines {
		cleanedLines = append(cleanedLines, bytes.TrimRight(line, " \t\r"))
	}

	// 2. Reconstruct with \n and then TrimSpace to handle any trailing blank lines
	newData := bytes.Join(cleanedLines, []byte("\n"))
	newData = bytes.TrimRight(newData, " \n\r\t")

	// 3. Ensure exactly one newline at EOF if not empty
	if len(newData) > 0 {
		newData = append(newData, '\n')
	}

	if !bytes.Equal(data, newData) {
		err = os.WriteFile(path, newData, 0644)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: clean_whitespace <file_or_dir>...")
		os.Exit(0)
	}

	var paths []string
	for _, arg := range os.Args[1:] {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error stating %s: %v\n", arg, err)
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
				// Filter common source files to avoid binary noise
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
			fmt.Fprintf(os.Stderr, "Error walking %s: %v\n", arg, err)
		}
	}

	var wg sync.WaitGroup
	mu := sync.Mutex{}
	modifiedCount := 0
	sem := make(chan struct{}, 20) // Concurrency limit

	for _, p := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			modified, err := cleanFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error cleaning %s: %v\n", path, err)
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
