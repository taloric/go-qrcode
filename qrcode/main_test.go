package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaxEncodablePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "numeric",
			input: strings.Repeat("1", 4000),
			want:  3057,
		},
		{
			name:  "alphanumeric",
			input: strings.Repeat("A", 1900),
			want:  1852,
		},
		{
			name:  "byte",
			input: strings.Repeat("#", 2000),
			want:  1273,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := maxEncodablePrefix([]rune(tt.input))
			if err != nil {
				t.Fatalf("maxEncodablePrefix returned error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("maxEncodablePrefix(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestSplitAndWriteCreatesNumberedFiles(t *testing.T) {
	t.Parallel()

	longContent := strings.Repeat("A", 1900)
	dir := t.TempDir()
	prefix := filepath.Join(dir, "qr")

	if err := splitAndWrite(longContent, 32, prefix, false, false, false); err != nil {
		t.Fatalf("splitAndWrite returned error: %v", err)
	}

	expected := []string{
		filepath.Join(dir, "qr-0.png"),
		filepath.Join(dir, "qr-1.png"),
	}

	for _, path := range expected {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s to exist: %v", path, err)
		}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(files) != len(expected) {
		t.Fatalf("unexpected number of files written: got %d, want %d", len(files), len(expected))
	}
}
