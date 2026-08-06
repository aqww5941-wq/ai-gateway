// Command quality hosts cross-platform repository quality checks used by CI.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

var formatRoots = []string{"cmd", "config", "internal"}

func main() {
	write := flag.Bool("write", false, "format Go files while preserving their checkout line endings")
	flag.Parse()

	unformatted, err := processGoFiles(formatRoots, *write)
	if err != nil {
		fmt.Fprintln(os.Stderr, "check Go formatting:", err)
		os.Exit(1)
	}
	if *write {
		for _, path := range unformatted {
			fmt.Println("formatted", path)
		}
		return
	}
	if len(unformatted) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "the following Go files are not gofmt-formatted:")
	for _, path := range unformatted {
		fmt.Fprintln(os.Stderr, path)
	}
	os.Exit(1)
}

func findUnformattedGoFiles(roots []string) ([]string, error) {
	return processGoFiles(roots, false)
}

func processGoFiles(roots []string, write bool) ([]string, error) {
	var unformatted []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			// The repository is used from Windows and Linux. CRLF is a checkout
			// concern, not a Go formatting difference, so compare normalized
			// source while still rejecting every structural gofmt change.
			normalized := bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
			formatted, err := format.Source(normalized)
			if err != nil {
				return fmt.Errorf("format %s: %w", path, err)
			}
			if !bytes.Equal(normalized, formatted) {
				unformatted = append(unformatted, filepath.ToSlash(path))
				if write {
					output := formatted
					crlfCount := bytes.Count(source, []byte("\r\n"))
					if crlfCount > 0 && crlfCount == bytes.Count(source, []byte("\n")) {
						output = bytes.ReplaceAll(formatted, []byte("\n"), []byte("\r\n"))
					}
					info, err := entry.Info()
					if err != nil {
						return fmt.Errorf("stat %s: %w", path, err)
					}
					if err := os.WriteFile(path, output, info.Mode().Perm()); err != nil {
						return fmt.Errorf("write %s: %w", path, err)
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(unformatted)
	return unformatted, nil
}
