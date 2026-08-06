package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindUnformattedGoFilesIgnoresOnlyLineEndings(t *testing.T) {
	root := t.TempDir()
	writeQualityFixture(t, root, "formatted-lf.go", "package fixture\n\nfunc ok() {}\n")
	writeQualityFixture(t, root, "formatted-crlf.go", "package fixture\r\n\r\nfunc ok() {}\r\n")
	writeQualityFixture(t, root, "unformatted.go", "package fixture\nfunc bad( ){ }\n")

	unformatted, err := findUnformattedGoFiles([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(unformatted) != 1 || !strings.HasSuffix(unformatted[0], "unformatted.go") {
		t.Fatalf("unformatted files = %#v", unformatted)
	}
}

func TestFindUnformattedGoFilesRejectsInvalidSource(t *testing.T) {
	root := t.TempDir()
	writeQualityFixture(t, root, "invalid.go", "package fixture\nfunc")

	_, err := findUnformattedGoFiles([]string{root})
	if err == nil || !strings.Contains(err.Error(), "invalid.go") {
		t.Fatalf("invalid source error = %v", err)
	}
}

func TestProcessGoFilesWritesFormatAndPreservesCRLF(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "unformatted.go")
	writeQualityFixture(t, root, "unformatted.go", "package fixture\r\nfunc bad( ){ }\r\n")

	changed, err := processGoFiles([]string{root}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 {
		t.Fatalf("changed files = %#v", changed)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != "package fixture\r\n\r\nfunc bad() {}\r\n" {
		t.Fatalf("formatted CRLF source = %q", source)
	}
	remaining, err := findUnformattedGoFiles([]string{root})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("remaining files = %#v, %v", remaining, err)
	}
}

func TestProcessGoFilesPreservesLF(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "unformatted.go")
	writeQualityFixture(t, root, "unformatted.go", "package fixture\nfunc bad( ){ }")

	if _, err := processGoFiles([]string{root}, true); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "\r\n") || string(source) != "package fixture\n\nfunc bad() {}\n" {
		t.Fatalf("formatted LF source = %q", source)
	}
}

func writeQualityFixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
