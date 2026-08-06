package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayBinaryName(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "windows", want: "gateway.exe"},
		{goos: "linux", want: "gateway"},
		{goos: "darwin", want: "gateway"},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := gatewayBinaryName(tt.goos); got != tt.want {
				t.Fatalf("gatewayBinaryName(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestFindRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "package.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findRepoRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("findRepoRoot() = %q, want %q", got, root)
	}
}

func TestFindRepoRootRejectsMissingMarkers(t *testing.T) {
	if _, err := findRepoRoot(t.TempDir()); err == nil {
		t.Fatal("findRepoRoot() error = nil, want repository marker error")
	}
}

func TestWithEnvReplacesExistingValueCaseInsensitively(t *testing.T) {
	got := withEnv([]string{"PATH=/bin", "cgo_enabled=1", "EMPTY="}, "CGO_ENABLED", "0")
	want := []string{"PATH=/bin", "EMPTY=", "CGO_ENABLED=0"}
	if len(got) != len(want) {
		t.Fatalf("withEnv() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("withEnv()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCleanLocalArtifactsPreservesEmbeddedDist(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "bin", "gateway"),
		filepath.Join(root, "web", "dist", "index.html"),
		filepath.Join(root, "internal", "static", "dist", "index.html"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := cleanLocalArtifacts(root); err != nil {
		t.Fatal(err)
	}
	for _, removed := range paths[:2] {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("cleaned path %q still exists or returned unexpected error: %v", removed, err)
		}
	}
	if _, err := os.Stat(paths[2]); err != nil {
		t.Fatalf("embedded dist was removed: %v", err)
	}
}
