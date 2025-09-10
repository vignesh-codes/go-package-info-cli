// Package app provides the main application logic for analyzing Debian package statistics.
package app

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/canonical-dev/package_statistics/internal/cache"
)

// PackageStats represents package file count statistics.
type PackageStats = cache.PackageStats // reuse struct

// CacheEntry represents a cached data entry.
type CacheEntry = cache.CacheEntry

// Config holds application configuration settings.
type Config struct {
	Architecture     string
	CacheDir         string
	CacheTTL         time.Duration
	ForceRefresh     bool
	TopCount         int
	ShortCacheWindow time.Duration
	DownloadTimeout  time.Duration
}

// App is the main application struct that handles package statistics analysis.
type App struct {
	client *http.Client
	cfg    *Config
	logger *log.Logger
}

// NewApp creates a new App instance with the given configuration and logger.
func NewApp(cfg *Config, logger *log.Logger) *App {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &App{
		// No timeout - allow streaming downloads with context cancellation
		client: &http.Client{},
		cfg:    cfg,
		logger: logger,
	}
}

// ParseFlags parses command line flags and returns a Config.
func ParseFlags() (*Config, error) {
	return parseFlags()
}

const (
	defaultCacheTTL        = 24 * time.Hour
	defaultCacheDir        = ".cache/package-statistics"
	defaultDownloadTimeout = 10 * time.Minute
	// BaseURL is the template URL for Debian package contents files.
	BaseURL = "http://ftp.uk.debian.org/debian/dists/stable/main/Contents-%s.gz"
	// MaxRetries is the maximum number of download retry attempts.
	MaxRetries = 3
)

// parseFlags handles the actual flag parsing logic.
func parseFlags() (*Config, error) {
	cacheTTL := flag.Duration("cache-ttl", defaultCacheTTL, "cache TTL")
	cacheDir := flag.String("cache-dir", defaultCacheDir, "cache directory")
	force := flag.Bool("force-refresh", false, "force refresh cache")
	top := flag.Int("top", 10, "number of top packages")
	downloadTimeout := flag.Duration("download-timeout", defaultDownloadTimeout, "download timeout (0 = no timeout)")
	help := flag.Bool("help", false, "show help")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		flag.Usage()
		return nil, fmt.Errorf("architecture argument required")
	}

	arch := strings.TrimSpace(flag.Arg(0))
	if arch == "" {
		return nil, fmt.Errorf("architecture cannot be empty")
	}

	dir, err := expandPath(*cacheDir)
	if err != nil {
		return nil, fmt.Errorf("invalid cache dir: %w", err)
	}

	return &Config{
		Architecture:     arch,
		CacheDir:         dir,
		CacheTTL:         *cacheTTL,
		ForceRefresh:     *force,
		TopCount:         *top,
		ShortCacheWindow: time.Hour,
		DownloadTimeout:  *downloadTimeout,
	}, nil
}

// expandPath expands ~ in file paths to the user's home directory.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}

/*
	AnalyzeWithCache orchestrates cache loading, download, and stats processing.

	a.cfg.ForceRefresh = true -> always download new data
	a.cfg.ForceRefresh = false -> use cached data if it exists and is recent

Step 1: Create cache file path and lock file path
Step 2: Acquire lock
Step 3: Load existing cache if exists
Step 4: Check if cache is recent enough (ShortCacheDuration is 1hr for now)
Step 5: Download new data if cache is not recent or if HEAD's request returns modified or cache doesn't exist
Step 6: Save cache if new data was downloaded
Step 7: Return stats
*/
func (a *App) AnalyzeWithCache(ctx context.Context) ([]PackageStats, error) {
	cacheFile := filepath.Join(a.cfg.CacheDir, fmt.Sprintf("contents-%s.json", a.cfg.Architecture))
	lockFile := cacheFile + ".lock"

	// cleanup old locks
	cache.CleanupStaleLock(lockFile, cache.LockStaleTTL)

	// acquire lock
	lock, err := cache.AcquireLockWithContext(ctx, lockFile, cache.LockTimeout)
	if err != nil {
		return nil, err
	}
	defer cache.ReleaseLock(lock, lockFile, a.logger)

	// load existing cache
	var cached *CacheEntry
	if !a.cfg.ForceRefresh {
		cached, _ = cache.LoadCache(cacheFile, a.cfg.CacheTTL)
	}

	// use short cache window
	if cached != nil && a.cfg.ShortCacheWindow > 0 && time.Since(cached.Timestamp) < a.cfg.ShortCacheWindow {
		a.logger.Printf("Using recent cached data (age=%s)", time.Since(cached.Timestamp).Truncate(time.Second))
		return cached.Stats, nil
	}

	// download new data with configurable timeout
	url := fmt.Sprintf(BaseURL, a.cfg.Architecture)
	downloadCtx := ctx
	if a.cfg.DownloadTimeout > 0 {
		var cancel context.CancelFunc
		downloadCtx, cancel = context.WithTimeout(ctx, a.cfg.DownloadTimeout)
		defer cancel()
	}
	stats, etag, lastMod, err := a.Download(downloadCtx, url, cached)
	if err != nil && cached != nil {
		if downloadCtx.Err() == context.DeadlineExceeded {
			a.logger.Printf("Download timeout after %v, falling back to cache", a.cfg.DownloadTimeout)
		} else {
			a.logger.Printf("Network error, falling back to cache: %v", err)
		}
		return cached.Stats, nil
	} else if err != nil {
		return nil, err
	}

	// save cache
	entry := &CacheEntry{
		Architecture: a.cfg.Architecture,
		Stats:        stats,
		Timestamp:    time.Now().UTC(),
		URL:          url,
		ETag:         etag,
		LastModified: lastMod,
	}

	if err := cache.SaveCache(cacheFile, entry); err != nil {
		a.logger.Printf("Failed to save cache: %v", err)
	}

	return stats, nil
}
