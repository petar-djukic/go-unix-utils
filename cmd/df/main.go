// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements df: report filesystem disk space usage.
// Implements srd106-df R1.1-R1.4, R4.1-R4.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const colCount = 6

// Column headers matching GNU df exactly (R1.2).
var headers = [colCount]string{
	"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on",
}

// rightAligned indicates which columns are right-aligned (R1.2).
var rightAligned = [colCount]bool{false, true, true, true, true, false}

// dummyTypes lists filesystem types that GNU df excludes by default.
var dummyTypes = map[string]bool{
	"autofs":    true,
	"devfs":     true,
	"fdescfs":   true,
	"linsysfs":  true,
	"linprocfs": true,
	"none":      true,
	"nullfs":    true,
	"procfs":    true,
}

// mountInfo holds filesystem data from the OS.
type mountInfo struct {
	source      string
	target      string
	fsType      string
	totalBlocks uint64
	freeBlocks  uint64
	availBlocks uint64
	blockSize   int64
}

// R4.3: install SIGPIPE handler at startup.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run parses arguments, collects filesystem data, and prints output.
// R4.1: returns 0 on success. R4.2: returns 1 on any error.
func run() int {
	paths := parseArgs(os.Args[1:])
	rows, hasError := collectEntries(paths)
	if len(rows) > 0 {
		printTable(rows)
	}
	if hasError {
		return 1
	}
	return 0
}

// parseArgs extracts FILE paths from command-line arguments.
// Accepts -k and --no-sync as no-ops per SRD non_goals.
func parseArgs(args []string) []string {
	var paths []string
	stopFlags := false
	for i := 0; i < len(args); i++ {
		if stopFlags || !strings.HasPrefix(args[i], "-") {
			paths = append(paths, args[i])
			continue
		}
		if args[i] == "--" {
			stopFlags = true
			continue
		}
		if isNoOpFlag(args[i]) {
			continue
		}
		paths = append(paths, args[i])
	}
	return paths
}

// isNoOpFlag returns true for flags accepted but without visible effect.
func isNoOpFlag(arg string) bool {
	return arg == "-k" || arg == "--no-sync"
}

// collectEntries gathers filesystem rows for the given paths.
// R1.1: no paths means all mounted filesystems.
// R1.4: paths means report filesystem containing each file.
func collectEntries(paths []string) ([][colCount]string, bool) {
	if len(paths) == 0 {
		return collectAllMounts()
	}
	return collectForPaths(paths)
}

// collectAllMounts returns rows for all non-pseudo mounted filesystems.
// R1.1: excludes pseudo-filesystems (0 total blocks) and dummy types.
// Deduplicates by device number to match GNU df filtering.
func collectAllMounts() ([][colCount]string, bool) {
	mounts, err := getMounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return nil, true
	}
	var rows [][colCount]string
	seenDev := make(map[uint64]bool)
	for _, m := range mounts {
		if skipMount(m, seenDev) {
			continue
		}
		rows = append(rows, mountToRow(m))
	}
	return rows, false
}

// skipMount returns true if a mount entry should be excluded from output.
// Filters: 0 total blocks, dummy types, and duplicate device numbers.
func skipMount(m mountInfo, seenDev map[uint64]bool) bool {
	if m.totalBlocks == 0 || dummyTypes[m.fsType] {
		return true
	}
	fi, err := sys.Stat(m.target)
	if err != nil {
		return false
	}
	if seenDev[fi.Dev] {
		return true
	}
	seenDev[fi.Dev] = true
	return false
}

// collectForPaths returns rows for filesystems containing the given files.
// R1.4: reports errors for non-existent files and continues.
func collectForPaths(paths []string) ([][colCount]string, bool) {
	var rows [][colCount]string
	hasError := false
	for _, p := range paths {
		m, err := getFilesystemInfo(p)
		if err != nil {
			reportError(p, err)
			hasError = true
			continue
		}
		rows = append(rows, mountToRow(*m))
	}
	return rows, hasError
}

// mountToRow converts filesystem data to a display row.
// R1.1: sizes in 1024-byte block units. R1.3: Filesystem source display.
func mountToRow(m mountInfo) [colCount]string {
	bs := m.blockSize
	total1K := int64(m.totalBlocks) * bs / 1024
	used1K := (int64(m.totalBlocks) - int64(m.freeBlocks)) * bs / 1024
	avail1K := int64(m.availBlocks) * bs / 1024
	usePct := computeUsePct(m.totalBlocks, m.freeBlocks, m.availBlocks)
	return [colCount]string{
		m.source,
		fmt.Sprintf("%d", total1K),
		fmt.Sprintf("%d", used1K),
		fmt.Sprintf("%d", avail1K),
		usePct,
		m.target,
	}
}

// computeUsePct calculates Use% as ceiling(used * 100 / (used + avail)).
// R1.3: returns "-" when the denominator is zero.
func computeUsePct(total, free, avail uint64) string {
	if total == 0 {
		return "-"
	}
	var used uint64
	if total > free {
		used = total - free
	}
	denom := used + avail
	if denom == 0 {
		return "-"
	}
	pct := (used*100 + denom - 1) / denom
	return fmt.Sprintf("%d%%", pct)
}

// printTable prints the header and all rows with aligned columns.
// R1.2: header and alignment matching GNU df.
func printTable(rows [][colCount]string) {
	widths := computeWidths(rows)
	printLine(headers, widths)
	for _, r := range rows {
		printLine(r, widths)
	}
}

// computeWidths returns the per-column max width including headers.
// R1.5: column widths are per-column maxima across all rows.
func computeWidths(rows [][colCount]string) [colCount]int {
	var widths [colCount]int
	for i := 0; i < colCount; i++ {
		widths[i] = len(headers[i])
	}
	for _, r := range rows {
		for i := 0; i < colCount; i++ {
			if len(r[i]) > widths[i] {
				widths[i] = len(r[i])
			}
		}
	}
	return widths
}

// printLine prints one row with proper alignment.
// R1.2: numeric columns right-aligned, text columns left-aligned.
func printLine(vals [colCount]string, widths [colCount]int) {
	var buf strings.Builder
	for i := 0; i < colCount; i++ {
		if i > 0 {
			buf.WriteByte(' ')
		}
		if i == colCount-1 {
			buf.WriteString(vals[i])
		} else if rightAligned[i] {
			fmt.Fprintf(&buf, "%*s", widths[i], vals[i])
		} else {
			fmt.Fprintf(&buf, "%-*s", widths[i], vals[i])
		}
	}
	buf.WriteByte('\n')
	fmt.Fprint(os.Stdout, buf.String())
}

// reportError prints a diagnostic to stderr for a file access error.
// R4.2: matches GNU df error format.
func reportError(path string, err error) {
	fmt.Fprintf(os.Stderr, "df: %s: %s\n", path, capitalizeFirst(errnoMsg(err)))
}

// errnoMsg extracts the underlying error message string.
func errnoMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
