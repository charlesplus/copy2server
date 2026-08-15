package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--file", "a.txt", "--no-copy", "--timeout", "2s", "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.copy {
		t.Fatal("copy = true, want false")
	}
	if opts.timeout != 2*time.Second {
		t.Fatalf("timeout = %v", opts.timeout)
	}
	if got := strings.Join(opts.files, ","); got != "a.txt,b.txt" {
		t.Fatalf("files = %s", got)
	}
}

func TestResolveServerURLPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("client.config.json", []byte(`{"serverUrl":"http://config:8282"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveServerURL(""); err != nil || got != "http://config:8282" {
		t.Fatalf("config server = %q, %v", got, err)
	}
	customConfig := filepath.Join(dir, "custom-client.json")
	if err := os.WriteFile(customConfig, []byte(`{"serverUrl":"http://custom:8282"}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COPY2SERVER_CONFIG", customConfig)
	if got, err := resolveServerURL(""); err != nil || got != "http://custom:8282" {
		t.Fatalf("custom config server = %q, %v", got, err)
	}
	t.Setenv("COPY2SERVER_CONFIG", "")
	t.Setenv("COPY2SERVER_URL", "http://env:8282")
	if got, err := resolveServerURL(""); err != nil || got != "http://env:8282" {
		t.Fatalf("env server = %q, %v", got, err)
	}
	if got, err := resolveServerURL("http://flag:8282/"); err != nil || got != "http://flag:8282" {
		t.Fatalf("flag server = %q, %v", got, err)
	}
}

func TestUploadParsesServerPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/upload" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.FormFile("file"); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]string{{"serverPath": "/srv/a.txt"}}})
	}))
	defer server.Close()

	paths, err := upload(contextWithTimeout(t), server.URL, []uploadCandidate{{filename: "a.txt", data: []byte("hello")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/srv/a.txt" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestRunExplicitFileNoCopy(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		_, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]string{{"serverPath": fmt.Sprintf("/srv/%s", header.Filename)}}})
	}))
	defer server.Close()

	var out strings.Builder
	var errOut strings.Builder
	if err := run([]string{"--server", server.URL, "--no-copy", file}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "/srv/a.txt" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestExistingFilesFromText(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	files := existingFilesFromText("\"" + file + "\"\nmissing.txt")
	if len(files) != 1 || files[0] != file {
		t.Fatalf("files = %#v", files)
	}
}

func contextWithTimeout(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}
