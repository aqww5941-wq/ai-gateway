package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadErrorGolden(t *testing.T) {
	tests := []string{"unknown-field", "invalid-strategy"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(filepath.Join("testdata", name+".yaml"))
			if err == nil {
				t.Fatal("Load() error = nil")
			}
			golden, readErr := os.ReadFile(filepath.Join("testdata", name+".golden"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := strings.TrimSpace(string(golden))
			if err.Error() != want {
				t.Fatalf("Load() error:\n%s\nwant:\n%s", err, want)
			}
		})
	}
}
