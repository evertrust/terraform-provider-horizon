//go:build e2e

// nolint
package tests

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedConfigFolder(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"2.7", "2.8", "2.10", "not-a-version"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		version string
		want    string
		wantErr bool
	}{
		{version: "2.7.21", want: "2.7"},
		{version: "2.8.4", want: "2.8"},
		{version: "2.9.5", want: "2.8"},
		{version: "2.10.7", want: "2.10"},
		{version: "2.11.0", want: "2.10"},
		{version: "3.0.0", want: "2.10"},
		{version: "2.6.0", wantErr: true},
		{version: "", wantErr: true},
		{version: "latest", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := seedConfigFolder(root, tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("seedConfigFolder(%q) = %q, want an error", tt.version, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("seedConfigFolder(%q): %v", tt.version, err)
			}
			if want := filepath.Join(root, tt.want); got != want {
				t.Fatalf("seedConfigFolder(%q) = %q, want %q", tt.version, got, want)
			}
		})
	}
}
