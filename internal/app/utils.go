package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/canonical-dev/package_statistics/internal/cache"
)

/*
ProcessLine parses a single line into counts
input line: "usr/bin/file1 pkg1,pkg2,pkg3"
output map: {"pkg1": 1, "pkg2": 1, "pkg3": 1}
*/
func ProcessLine(line string, m map[string]int) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "FILE") {
		return
	}
	idx := strings.Index(line, " ")
	if idx == -1 {
		return
	}
	for _, pkg := range strings.Split(strings.TrimSpace(line[idx+1:]), ",") {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			m[pkg]++ // increments the count outside of this function
		}
	}
}

// SortMap converts map to sorted slice
func SortMap(m map[string]int) []cache.PackageStats {
	stats := make([]cache.PackageStats, 0, len(m))
	for k, v := range m {
		stats = append(stats, cache.PackageStats{Name: k, FileCount: v})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].FileCount > stats[j].FileCount })
	return stats
}

// PrintTop displays top packages with rank
func PrintTop(stats []cache.PackageStats, top int) {
	if len(stats) < top {
		top = len(stats)
	}

	fmt.Printf("%-5s %-30s %s\n", "Rank", "Package Name", "Count")
	fmt.Println(strings.Repeat("-", 50))

	for i := 0; i < top; i++ {
		// Clean package name by replacing tabs with spaces and trimming whitespace
		// Contents-source.gz had tabs "\t" in the package name
		cleanName := strings.ReplaceAll(stats[i].Name, "\t", " ")
		cleanName = strings.TrimSpace(cleanName)

		fmt.Printf("%-5d %-40s %d\n", i+1, cleanName, stats[i].FileCount)
	}
}
