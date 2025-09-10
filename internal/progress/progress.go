// Package progress provides a simple progress bar for file downloads.
package progress

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// ProgressReader wraps an io.Reader and displays download progress.
type ProgressReader struct {
	Reader    io.Reader
	Total     int64
	Curr      int64
	Last      time.Time
	StartTime time.Time
	Logger    func(string, ...interface{})
}

// Read implements io.Reader and updates the progress bar.
func (p *ProgressReader) Read(b []byte) (int, error) {
	// Initialize start time on first read
	if p.StartTime.IsZero() {
		p.StartTime = time.Now()
		p.Last = p.StartTime
	}

	n, err := p.Reader.Read(b)
	if n > 0 {
		p.Curr += int64(n)
		if time.Since(p.Last) > 500*time.Millisecond {
			p.render()
			p.Last = time.Now()
		}
	}
	if err == io.EOF {
		p.render()
		if p.Logger != nil {
			p.Logger("Download completed")
		} else {
			fmt.Println()
		}
	}
	return n, err
}

// render displays the current progress bar with download speed and ETA.
func (p *ProgressReader) render() {
	elapsed := time.Since(p.StartTime)
	speed := float64(p.Curr) / elapsed.Seconds()
	speedMB := speed / (1024 * 1024)
	currMB := float64(p.Curr) / (1024 * 1024)

	if p.Total == 0 {
		// Unknown total size - show only downloaded amount and speed
		fmt.Printf("\rDownloading: %.1f MB downloaded (%.1f MB/s)", currMB, speedMB)
		return
	}

	percent := float64(p.Curr) / float64(p.Total) * 100
	bar := strings.Repeat("█", int(percent/2)) + strings.Repeat(" ", 50-int(percent/2))

	// Calculate ETA
	remaining := float64(p.Total-p.Curr) / speed
	eta := time.Duration(remaining) * time.Second

	// Format sizes
	totalMB := float64(p.Total) / (1024 * 1024)

	// Always use direct stdout output for progress bar to enable in-place updates
	fmt.Printf("\r[%s] %6.2f%% (%.1f/%.1f MB, %.1f MB/s, ETA: %v)",
		bar, percent, currMB, totalMB, speedMB, eta.Truncate(time.Second))
}
