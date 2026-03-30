// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements df: report filesystem disk space usage.
// Implements prd106-df R1.1, R1.2, R1.3, R1.4, R4.1, R4.2, R4.3.
package main

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// fsEntry holds filesystem statistics for a single mount point.
type fsEntry struct {
	source    string
	fsType    string
	blocks1K  int64
	used      int64
	available int64
	mountedOn string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes df logic and returns the exit code.
func run(args []string) int {
	entries, exitCode := collectEntries(args)
	printDefault(entries)
	return exitCode
}

// collectEntries gathers filesystem entries from args or all mounts.
func collectEntries(args []string) ([]fsEntry, int) {
	if len(args) == 0 {
		return collectAllFilesystems()
	}
	return collectFileArgs(args)
}

// collectAllFilesystems returns all mounted non-pseudo filesystems.
// R1.1: exclude pseudo-filesystems (those with 0 total blocks).
func collectAllFilesystems() ([]fsEntry, int) {
	entries, err := enumerateFilesystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return nil, 1
	}
	return filterPseudo(entries), 0
}

// collectFileArgs returns filesystem info for each specified path.
// R1.4: report only the filesystem containing each FILE.
func collectFileArgs(args []string) ([]fsEntry, int) {
	var entries []fsEntry
	exitCode := 0
	for _, path := range args {
		entry, err := statfsForPath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "df: '%s': %v\n", path, err)
			exitCode = 1
			continue
		}
		entries = append(entries, *entry)
	}
	return entries, exitCode
}

// filterPseudo removes filesystems with 0 total blocks.
func filterPseudo(entries []fsEntry) []fsEntry {
	result := make([]fsEntry, 0, len(entries))
	for _, e := range entries {
		if e.blocks1K > 0 {
			result = append(result, e)
		}
	}
	return result
}

// computeUsePct calculates the use percentage matching GNU df.
// R1.3: ceiling(used * 100 / (used + available)) when denominator > 0.
func computeUsePct(used, available int64) string {
	denom := used + available
	if denom <= 0 {
		return "-"
	}
	pct := int(math.Ceil(float64(used) * 100.0 / float64(denom)))
	return fmt.Sprintf("%d%%", pct)
}

// printDefault outputs the default df format with aligned columns.
// R1.2: header and numeric column right-alignment.
func printDefault(entries []fsEntry) {
	headers := []string{
		"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on",
	}
	rows := buildDefaultRows(entries)
	widths := computeColumnWidths(headers, rows)
	printAlignedRow(headers, widths)
	for _, row := range rows {
		printAlignedRow(row, widths)
	}
}

// buildDefaultRows converts filesystem entries to string rows.
func buildDefaultRows(entries []fsEntry) [][]string {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			e.source,
			fmt.Sprintf("%d", e.blocks1K),
			fmt.Sprintf("%d", e.used),
			fmt.Sprintf("%d", e.available),
			computeUsePct(e.used, e.available),
			e.mountedOn,
		}
	}
	return rows
}

// computeColumnWidths returns the maximum width per column.
// R1.5: per-column maxima across all rows including the header.
func computeColumnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}

// printAlignedRow prints a single row with proper alignment.
// Columns 1-4 (1K-blocks, Used, Available, Use%) are right-aligned.
// Columns 0, 5 (Filesystem, Mounted on) are left-aligned.
func printAlignedRow(cells []string, widths []int) {
	parts := make([]string, len(cells))
	lastIdx := len(cells) - 1
	for i, cell := range cells {
		switch {
		case i >= 1 && i <= 4:
			parts[i] = fmt.Sprintf("%*s", widths[i], cell)
		case i == lastIdx:
			parts[i] = cell
		default:
			parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		}
	}
	fmt.Println(strings.Join(parts, " "))
}
