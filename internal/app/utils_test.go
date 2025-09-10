package app

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/canonical-dev/package_statistics/internal/cache"
)

func TestProcessLine(t *testing.T) {
	m := make(map[string]int)
	ProcessLine("usr/bin/file1 pkg1,pkg2,pkg3", m)

	if m["pkg1"] != 1 || m["pkg2"] != 1 || m["pkg3"] != 1 {
		t.Errorf("got %v", m)
	}
}

func TestProcessLineEdgeCases(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"", 0},
		{"FILE header", 0},
		{"usr/bin/file1pkg1", 0}, // no space
		{"usr/bin/file1 single-pkg", 1},
	}

	for _, tt := range tests {
		m := make(map[string]int)
		ProcessLine(tt.line, m)
		if len(m) != tt.want {
			t.Errorf("line %q: got %d packages, want %d", tt.line, len(m), tt.want)
		}
	}
}

func TestSortMap(t *testing.T) {
	m := map[string]int{
		"pkg-low":  5,
		"pkg-high": 50,
	}

	stats := SortMap(m)

	if len(stats) != 2 {
		t.Errorf("got %d entries", len(stats))
	}
	// Should be sorted by count descending
	if stats[0].Name != "pkg-high" || stats[0].FileCount != 50 {
		t.Errorf("got %+v", stats[0])
	}
}

func TestPrintTop(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = w

	stats := []cache.PackageStats{{Name: "pkg1", FileCount: 100}}
	PrintTop(stats, 5)
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "pkg1") {
		t.Error("missing pkg1")
	}
}
