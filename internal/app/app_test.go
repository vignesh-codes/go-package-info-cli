package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/canonical-dev/package_statistics/internal/cache"
)

func TestExpandPath(t *testing.T) {
	got, err := expandPath("~/testpath")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "testpath") {
		t.Errorf("got %s", got)
	}
}

func TestParseFlagsNoArch(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	old := flag.CommandLine
	defer func() { flag.CommandLine = old }()
	flag.CommandLine = fs

	fs.Parse([]string{})
	_, err := parseFlags()
	if err == nil {
		t.Fatal("should fail without arch")
	}
}

func TestDownload(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	fmt.Fprintln(gz, "usr/bin/file1 pkg1,pkg2")
	fmt.Fprintln(gz, "usr/lib/file2 pkg1")
	gz.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "test-etag")
		w.Write(buf.Bytes())
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

func TestDownloadNotModified(t *testing.T) {
	cached := &cache.CacheEntry{
		Stats:        []cache.PackageStats{{Name: "cached-pkg", FileCount: 50}},
		ETag:         "test-etag",
		LastModified: "test-lastmod",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	app := NewApp(&Config{Architecture: "amd64", CacheDir: t.TempDir()}, nil)
	stats, etag, _, err := app.Download(context.Background(), server.URL, cached)

	if err != nil {
		t.Fatal(err)
	}
	if stats[0].Name != "cached-pkg" {
		t.Errorf("got %s", stats[0].Name)
	}
	if etag != "test-etag" {
		t.Errorf("got etag %s", etag)
	}
}

func TestCacheHit(t *testing.T) {
	tempDir := t.TempDir()
	entry := &cache.CacheEntry{
		Architecture: "amd64",
		Stats:        []cache.PackageStats{{Name: "cached-pkg", FileCount: 100}},
		Timestamp:    time.Now().UTC(),
		URL:          "http://example.com/test", // Add URL to prevent real network call
	}

	cacheFile := fmt.Sprintf("%s/contents-amd64.json", tempDir)
	cache.SaveCache(cacheFile, entry)

	app := NewApp(&Config{
		Architecture:     "amd64",
		CacheDir:         tempDir,
		CacheTTL:         time.Hour,
		ShortCacheWindow: time.Minute, // Add this to use cache
	}, nil)

	stats, err := app.AnalyzeWithCache(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats[0].Name != "cached-pkg" {
		t.Errorf("got %s", stats[0].Name)
	}
}

func TestNewApp(t *testing.T) {
	cfg := &Config{Architecture: "amd64", CacheDir: "/tmp"}
	app := NewApp(cfg, nil)

	if app.cfg != cfg {
		t.Error("config not set")
	}
	if app.logger == nil {
		t.Error("logger not created")
	}
}
