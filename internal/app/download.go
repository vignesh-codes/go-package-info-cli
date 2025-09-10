package app

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical-dev/package_statistics/internal/cache"
	"github.com/canonical-dev/package_statistics/internal/progress"
)

// Download fetches and parses package statistics from a URL with caching support.
func (a *App) Download(ctx context.Context, url string, cached *cache.CacheEntry) ([]cache.PackageStats, string, string, error) {
	var etag, lastMod string

	// Step 1: HEAD
	headResp, err := HeadRequest(ctx, a.client, url, cached)
	if err == nil {
		defer headResp.Body.Close()
		etag = headResp.Header.Get("ETag")
		lastMod = headResp.Header.Get("Last-Modified")

		if cached != nil && (headResp.StatusCode == http.StatusNotModified ||
			(etag == cached.ETag && lastMod == cached.LastModified)) {
			a.logger.Printf("Using cached data")
			return cached.Stats, cached.ETag, cached.LastModified, nil
		}
	} else {
		a.logger.Printf("HEAD request failed: %v; falling back to GET", err)
	}

	// Step 2: GET with retries
	a.logger.Printf("Starting download from %s", url)
	resp, err := GetRequestWithRetry(ctx, a.client, url, cached)
	if err != nil {
		if cached != nil {
			a.logger.Printf("GET request failed, using cache: %v", err)
			return cached.Stats, cached.ETag, cached.LastModified, nil
		}
		return nil, "", "", err
	}
	defer resp.Body.Close()

	// Log download info
	if resp.ContentLength > 0 {
		a.logger.Printf("Downloading %d bytes (%.1f MB)", resp.ContentLength, float64(resp.ContentLength)/(1024*1024))
	} else {
		a.logger.Printf("Downloading (size unknown)")
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		if cached != nil {
			return cached.Stats, cached.ETag, cached.LastModified, nil
		}
		return nil, "", "", fmt.Errorf("304 received but no cache")
	case http.StatusNotFound:
		return nil, "", "", fmt.Errorf("404: Requested Package Contents Not Found: %s", url)
	default:
		return nil, "", "", fmt.Errorf("HTTP %d at %s", resp.StatusCode, url)
	}

	etag = resp.Header.Get("ETag")
	lastMod = resp.Header.Get("Last-Modified")

	// Parse body with enhanced progress reporting
	pr := &progress.ProgressReader{
		Reader: resp.Body,
		Total:  resp.ContentLength,
		Logger: a.logger.Printf,
	}
	gz, err := gzip.NewReader(pr)
	if err != nil {
		return nil, "", "", err
	}
	defer gz.Close()

	// counts is a map of package name to file count
	// sample: {"pkg1": 1, "pkg2": 1, "pkg3": 1}
	counts := make(map[string]int)
	// scanner is a bufio.Scanner that reads the gzip-compressed contents
	// sample: "usr/bin/file1 pkg1,pkg2,pkg3"
	scanner := bufio.NewScanner(gz)
	// buf is a buffer for the scanner - 10MB
	// 10MB -> if the file is larger than 10MB, we will need to read it in chunks
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lineCount := 0
	// Scan the file line by line
	for scanner.Scan() {
		// Check for cancellation every 1000 lines for responsiveness
		if lineCount%1000 == 0 {
			if ctx.Err() != nil {
				a.logger.Printf("Download cancelled by user: %v", ctx.Err())
				return nil, "", "", ctx.Err()
			}
		}
		// Process the line into the counts map
		// scanner.Text() is the line - "usr/bin/file1 pkg_names"
		ProcessLine(scanner.Text(), counts)
		lineCount++
	}
	if scanner.Err() != nil {
		return nil, "", "", scanner.Err()
	}
	// Sort the counts map
	return SortMap(counts), etag, lastMod, nil
}

// HeadRequest performs HEAD request with ETag/Last-Modified headers
func HeadRequest(ctx context.Context, client *http.Client, url string, cached *CacheEntry) (*http.Response, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if cached != nil {
		if cached.ETag != "" {
			req.Header.Set("If-None-Match", cached.ETag)
		}
		if cached.LastModified != "" {
			req.Header.Set("If-Modified-Since", cached.LastModified)
		}
	}
	return client.Do(req)
}

// GetRequestWithRetry performs GET request with retries
func GetRequestWithRetry(ctx context.Context, client *http.Client, url string, cached *CacheEntry) (*http.Response, error) {
	var resp *http.Response
	var err error
	for i := 0; i < MaxRetries; i++ {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if cached != nil {
			if cached.ETag != "" {
				req.Header.Set("If-None-Match", cached.ETag)
			}
			if cached.LastModified != "" {
				req.Header.Set("If-Modified-Since", cached.LastModified)
			}
		}
		resp, err = client.Do(req)
		if err == nil {
			return resp, nil
		}

		// Don't sleep on last retry or if context cancelled
		if i < MaxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second * (1 << i)):
				// Continue to next retry
			}
		}
	}
	return nil, err
}
