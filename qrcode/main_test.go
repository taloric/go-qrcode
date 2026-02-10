package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qrcode "github.com/skip2/go-qrcode"
)

func TestSplitAndWriteCreatesNumberedFiles(t *testing.T) {
	t.Parallel()

	longContent := strings.Repeat("A", 1900)
	dir := t.TempDir()
	prefix := filepath.Join(dir, "qr")

	if err := splitAndWrite(longContent, 32, prefix, false, false, false, false); err != nil {
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

func TestSplitAndWriteGrid(t *testing.T) {
	t.Parallel()

	longContent := strings.Repeat("#", 3000)
	dir := t.TempDir()
	prefix := filepath.Join(dir, "qr")

	if err := splitAndWrite(longContent, 32, prefix, false, false, false, true); err != nil {
		t.Fatalf("splitAndWrite grid returned error: %v", err)
	}

	gridFile := filepath.Join(dir, "qr-grid.png")
	if _, err := os.Stat(gridFile); err != nil {
		t.Fatalf("expected grid file %s to exist: %v", gridFile, err)
	}
}

func TestMaxByteCapacity(t *testing.T) {
	t.Parallel()

	// Version 40 Highest: (10208 - 20) / 8 = 1273
	if got := qrcode.MaxByteCapacity(qrcode.Highest); got != 1273 {
		t.Fatalf("MaxByteCapacity(Highest) = %d, want 1273", got)
	}
}

func TestDecodePNGDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Encode a short message to PNG
	content := "hello world test"
	q, err := qrcode.New(content, qrcode.Highest)
	if err != nil {
		t.Fatalf("qrcode.New failed: %v", err)
	}

	png, err := q.PNG(256)
	if err != nil {
		t.Fatalf("PNG failed: %v", err)
	}

	pngPath := filepath.Join(dir, "test.png")
	if err := os.WriteFile(pngPath, png, 0644); err != nil {
		t.Fatalf("write PNG failed: %v", err)
	}

	// Decode
	if err := decodePNGDir(dir); err != nil {
		t.Fatalf("decodePNGDir failed: %v", err)
	}

	// Verify decoded text
	txtPath := filepath.Join(dir, "test.txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read decoded txt failed: %v", err)
	}

	if string(data) != content {
		t.Fatalf("decoded content mismatch: got %q, want %q", string(data), content)
	}
}

func TestDecodePNGFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "single file decode test"
	q, err := qrcode.New(content, qrcode.Highest)
	if err != nil {
		t.Fatalf("qrcode.New failed: %v", err)
	}

	png, err := q.PNG(256)
	if err != nil {
		t.Fatalf("PNG failed: %v", err)
	}

	pngPath := filepath.Join(dir, "single.png")
	if err := os.WriteFile(pngPath, png, 0644); err != nil {
		t.Fatalf("write PNG failed: %v", err)
	}

	// Decode single file via decodePNG (unified entry)
	if err := decodePNG(pngPath); err != nil {
		t.Fatalf("decodePNG single file failed: %v", err)
	}

	txtPath := filepath.Join(dir, "single.txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read decoded txt failed: %v", err)
	}

	if string(data) != content {
		t.Fatalf("decoded content mismatch: got %q, want %q", string(data), content)
	}
}

func TestDecodeSmallImage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Short content = low version QR code, small image where scale-up helps
	content := "scale up test"
	q, err := qrcode.New(content, qrcode.Highest)
	if err != nil {
		t.Fatalf("qrcode.New failed: %v", err)
	}

	// Very small: 64px for a low-version QR code
	png, err := q.PNG(64)
	if err != nil {
		t.Fatalf("PNG failed: %v", err)
	}

	pngPath := filepath.Join(dir, "small.png")
	if err := os.WriteFile(pngPath, png, 0644); err != nil {
		t.Fatalf("write PNG failed: %v", err)
	}

	// scaleUpForDecode should scale this up for reliable decoding
	if err := decodePNG(pngPath); err != nil {
		t.Fatalf("decodePNG small image failed: %v", err)
	}

	txtPath := filepath.Join(dir, "small.txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read decoded txt failed: %v", err)
	}

	if string(data) != content {
		t.Fatalf("decoded content mismatch: got %q, want %q", string(data), content)
	}
}

func TestRoundTripSplitDecode(t *testing.T) {
	t.Parallel()

	// Encode long content into multiple QR codes
	original := strings.Repeat("ABCDEF0123456789", 200) // 3200 bytes
	dir := t.TempDir()
	prefix := filepath.Join(dir, "chunk")

	// Large fixed size ensures both high-version and low-version QR codes are decodable
	if err := splitAndWrite(original, 2048, prefix, false, false, false, false); err != nil {
		t.Fatalf("splitAndWrite failed: %v", err)
	}

	// Decode all QR codes
	if err := decodePNGDir(dir); err != nil {
		t.Fatalf("decodePNGDir failed: %v", err)
	}

	// Read and concatenate decoded text in order
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var parts []string
	for i := 0; ; i++ {
		txtName := filepath.Join(dir, "chunk-"+itoa(i)+".txt")
		data, err := os.ReadFile(txtName)
		if err != nil {
			break
		}
		parts = append(parts, string(data))
	}

	reconstructed := strings.Join(parts, "")
	if reconstructed != original {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(reconstructed), len(original))
	}

	_ = entries
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func TestLoadContentFromFileHex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	data := []byte{0x00, 0xff, 0x10}

	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}

	content, err := loadContent(nil, path)
	if err != nil {
		t.Fatalf("loadContent returned error: %v", err)
	}

	if content != "00ff10" {
		t.Fatalf("hex encoding mismatch, got %s", content)
	}
}

func TestLoadContentConflicts(t *testing.T) {
	t.Parallel()

	if _, err := loadContent([]string{"foo"}, "bar"); err == nil {
		t.Fatalf("expected error when both args and file provided")
	}
}

func TestLoadContentNoInput(t *testing.T) {
	t.Parallel()

	if _, err := loadContent(nil, ""); err == nil {
		t.Fatalf("expected error when no input provided")
	}
}
