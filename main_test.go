package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizeFilenamePreservesExtension(t *testing.T) {
	tests := map[string]string{
		"note.txt":    "note.txt",
		"测试.txt":      "file.txt",
		"新建 文本文档.TXT": "file.txt",
		"a b.txt":     "a-b.txt",
	}

	for input, want := range tests {
		if got := sanitizeFilename(input); got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLoadConfigStorageDefaultsAndEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"maxStorageGB":0,"maxUploadMB":0,"retentionDays":0}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG", configPath)
	t.Setenv("MAX_STORAGE_GB", "7.5")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxStorageGB != 7.5 {
		t.Fatalf("MaxStorageGB = %v, want 7.5", cfg.MaxStorageGB)
	}
	if cfg.MaxUploadMB != defaultMaxUploadMB {
		t.Fatalf("MaxUploadMB = %d, want default", cfg.MaxUploadMB)
	}
	if cfg.RetentionDays != defaultRetentionDays {
		t.Fatalf("RetentionDays = %d, want default", cfg.RetentionDays)
	}
}

func TestCleanupFilesRetentionAndQuota(t *testing.T) {
	dir := t.TempDir()
	old := writeSizedFile(t, dir, "old.txt", 1)
	mid := writeSizedFile(t, dir, "mid.txt", 4)
	newest := writeSizedFile(t, dir, "new.txt", 4)
	now := time.Now()
	mustChtimes(t, old, now.Add(-2*time.Hour))
	mustChtimes(t, mid, now.Add(-10*time.Minute))
	mustChtimes(t, newest, now)

	a := &app{uploadDirAbs: dir, retention: time.Hour, maxStorage: 6}
	if err := a.cleanupFiles(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old file still exists or stat err = %v", err)
	}
	if _, err := os.Stat(mid); !os.IsNotExist(err) {
		t.Fatalf("mid file still exists or stat err = %v", err)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Fatalf("newest file should remain: %v", err)
	}
}

func TestHandleUploadRejectsFileLargerThanStorageQuota(t *testing.T) {
	dir := t.TempDir()
	a := &app{uploadDirAbs: dir, maxUpload: 1024, maxStorage: 4, retention: 24 * time.Hour}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	a.handleUpload(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body %s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no saved files, got %d", len(entries))
	}
}

func writeSizedFile(t *testing.T, dir, name string, size int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustChtimes(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}
