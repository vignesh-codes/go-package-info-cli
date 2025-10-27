package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/canonical-dev/package_statistics/internal/cache"
)

func TestDownloadSuccess(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	fmt.Fprintln(gz, "usr/bin/file1 pkg1,pkg2")
	fmt.Fprintln(gz, "usr/lib/file2 pkg1")
	gz.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "test-etag")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	app := NewApp(&Config{Architecture: "amd64", CacheDir: t.TempDir()}, nil)
	stats, etag, _, err := app.Download(context.Background(), server.URL, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Errorf("got %d packages", len(stats))
	}
	if etag != "test-etag" {
		t.Errorf("got etag %s", etag)
	}
}

func TestDownloadCacheMatch(t *testing.T) {
	cached := &cache.CacheEntry{
		Stats:        []cache.PackageStats{{Name: "cached-pkg", FileCount: 100}},
		ETag:         "same-etag",
		LastModified: "same-lastmod",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("ETag", "same-etag")
			w.Header().Set("Last-Modified", "same-lastmod")
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Error("should not make GET request")
	}))
	defer server.Close()

	app := NewApp(&Config{Architecture: "amd64", CacheDir: t.TempDir()}, nil)
	stats, _, _, err := app.Download(context.Background(), server.URL, cached)

	if err != nil {
		t.Fatal(err)
	}
	if stats[0].Name != "cached-pkg" {
		t.Errorf("got %s", stats[0].Name)
	}
}

func TestDownloadErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{"not found", 404, "404"},
		{"server error", 500, "HTTP 500"},
	}

	for _, tt := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
		}))
		defer server.Close()

		app := NewApp(&Config{Architecture: "amd64", CacheDir: t.TempDir()}, nil)
		_, _, _, err := app.Download(context.Background(), server.URL, nil)

		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: got %v, want %s", tt.name, err, tt.want)
		}
	}
}

func TestDownloadNetworkFallback(t *testing.T) {
	cached := &cache.CacheEntry{
		Stats: []cache.PackageStats{{Name: "fallback-pkg", FileCount: 75}},
	}

	app := NewApp(&Config{Architecture: "amd64", CacheDir: t.TempDir()}, nil)
	stats, _, _, err := app.Download(context.Background(), "http://invalid-host.local", cached)

	if err != nil {
		t.Fatal(err)
	}
	if stats[0].Name != "fallback-pkg" {
		t.Errorf("got %s", stats[0].Name)
	}
}
