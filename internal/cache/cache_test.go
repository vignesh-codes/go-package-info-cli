package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackageStats(t *testing.T) {
	stats := PackageStats{Name: "test-pkg", FileCount: 42}
	if stats.Name != "test-pkg" || stats.FileCount != 42 {
		t.Errorf("got %+v", stats)
	}
}

func TestSaveCache(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "test.json")
	entry := &CacheEntry{
		Architecture: "amd64",
		Stats:        []PackageStats{{Name: "pkg1", FileCount: 10}},
		Timestamp:    time.Now().UTC(),
	}

	err := SaveCache(cacheFile, entry)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Checksum == "" {
		t.Error("checksum not set")
	}
}

func TestSaveCacheInvalidDir(t *testing.T) {
	entry := &CacheEntry{Architecture: "amd64", Stats: []PackageStats{}}
	err := SaveCache("/invalid/path/cache.json", entry)
	if err == nil {
		t.Fatal("should fail")
	}
}

func TestLoadCache(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "test.json")
	entry := &CacheEntry{
		Architecture: "amd64",
		Stats:        []PackageStats{{Name: "pkg1", FileCount: 15}},
		Timestamp:    time.Now().UTC(),
	}

	SaveCache(cacheFile, entry)
	loaded, err := LoadCache(cacheFile, time.Hour)

	if err != nil {
		t.Fatal(err)
	}
	if loaded.Architecture != "amd64" {
		t.Errorf("got %s", loaded.Architecture)
	}
}

func TestLoadCacheExpired(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "expired.json")
	entry := &CacheEntry{
		Architecture: "amd64",
		Stats:        []PackageStats{{Name: "pkg1", FileCount: 1}},
		Timestamp:    time.Now().UTC().Add(-2 * time.Hour),
	}

	SaveCache(cacheFile, entry)
	_, err := LoadCache(cacheFile, time.Hour)

	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("got %v", err)
	}
}

func TestLoadCacheCorrupt(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "corrupt.json")
	os.WriteFile(cacheFile, []byte("invalid json"), 0644)

	_, err := LoadCache(cacheFile, time.Hour)
	if err == nil {
		t.Fatal("should fail")
	}
}

func TestCleanupStaleLock(t *testing.T) {
	lockFile := filepath.Join(t.TempDir(), "test.lock")
	os.WriteFile(lockFile, []byte("lock"), 0644)

	oldTime := time.Now().Add(-2 * time.Hour)
	os.Chtimes(lockFile, oldTime, oldTime)

	CleanupStaleLock(lockFile, time.Hour)

	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Error("should remove stale lock")
	}
}

func TestLocks(t *testing.T) {
	lockFile := filepath.Join(t.TempDir(), "test.lock")

	lock, err := AcquireLock(lockFile, LockTimeout)
	if err != nil {
		t.Fatal(err)
	}

	ReleaseLock(lock, lockFile, nil)

	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Error("lock file should be removed")
	}
}
