// Package cache provides file-based caching with locking for package statistics.
package cache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofrs/flock"
)

const (
	// DefaultCacheTTL is the default cache time-to-live duration.
	DefaultCacheTTL = 24 * time.Hour

	// LockTimeout is how long to wait for a file lock.
	LockTimeout = 30 * time.Second
	// LockStaleTTL is when to consider a lock file stale and remove it.
	LockStaleTTL = 1 * time.Hour
)

// PackageStats holds the name and file count for a package.
type PackageStats struct {
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
}

// CacheEntry represents a complete cache entry with metadata.
type CacheEntry struct {
	Architecture string         `json:"architecture"`
	Stats        []PackageStats `json:"stats"`
	Timestamp    time.Time      `json:"timestamp"`
	ETag         string         `json:"etag,omitempty"`
	LastModified string         `json:"last_modified,omitempty"`
	URL          string         `json:"url"`
	Checksum     string         `json:"checksum,omitempty"`
}

// LoadCache loads JSON cache and validates TTL
func LoadCache(file string, ttl time.Duration) (*CacheEntry, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = os.Remove(file)
		return nil, fmt.Errorf("corrupt cache removed")
	}
	if time.Since(entry.Timestamp) > ttl {
		return nil, fmt.Errorf("cache expired")
	}
	return &entry, nil
}

// SaveCache writes JSON cache safely with checksum
func SaveCache(file string, entry *CacheEntry) error {
	data, err := json.Marshal(entry.Stats)
	if err != nil {
		return err
	}
	// we are not handling checksum logics for now
	entry.Checksum = fmt.Sprintf("%x", md5.Sum(data))

	tmp := file + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
		_ = os.Remove(tmp)
	}()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entry); err != nil {
		return err
	}

	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		err := os.Rename(tmp, file)
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("failed to rename tmp cache file: %s", file)
}

// CleanupStaleLock removes old lock files
func CleanupStaleLock(file string, ttl time.Duration) {
	if info, err := os.Stat(file); err == nil && time.Since(info.ModTime()) > ttl {
		_ = os.Remove(file)
	}
}

// AcquireLock gets a file lock with timeout
func AcquireLock(file string, timeout time.Duration) (*flock.Flock, error) {
	return AcquireLockWithContext(context.Background(), file, timeout)
}

// AcquireLockWithContext gets a file lock with timeout and context cancellation support
func AcquireLockWithContext(ctx context.Context, file string, timeout time.Duration) (*flock.Flock, error) {
	f := flock.New(file)
	locked, err := f.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		lockCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		locked, err = f.TryLockContext(lockCtx, timeout)
		if err != nil || !locked {
			return nil, err
		}
	}
	return f, nil
}

// ReleaseLock unlocks and deletes lock file
func ReleaseLock(f *flock.Flock, file string, logger *log.Logger) {
	if f == nil {
		return
	}
	if err := f.Unlock(); err != nil && logger != nil {
		logger.Printf("Failed to release lock: %v", err)
	}
	if err := os.Remove(file); err != nil && logger != nil {
		logger.Printf("Failed to remove lock file: %v", err)
	}
}
