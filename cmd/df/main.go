// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd106-df R1.1, R1.2, R1.3, R1.4 -- df core filesystem queries.
// Reports filesystem disk space usage for mounted filesystems.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// fsInfo holds filesystem statistics from a platform-specific query.
type fsInfo struct {
	Device      string
	MountPoint  string
	FSType      string
	TotalBlocks uint64
	BlockSize   uint64
	FreeBlocks  uint64
	AvailBlocks uint64
	TotalInodes uint64
	FreeInodes  uint64
}

// R1.2: column headers matching GNU df exactly.
var defaultHeader = []string{
	"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on",
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return listAllFilesystems()
	}
	return listPathFilesystems(args)
}

// R1.1: list all mounted filesystems, excluding pseudo-filesystems.
func listAllFilesystems() int {
	all, err := getAllFilesystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries := filterPseudo(all)
	printTable(entries)
	return 0
}

// R1.4: report the filesystem containing each FILE argument.
func listPathFilesystems(args []string) int {
	exitCode := 0
	var entries []fsInfo
	for _, path := range args {
		info, err := getPathFilesystem(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "df: %s: %v\n", path, err)
			exitCode = 1
			continue
		}
		entries = append(entries, *info)
	}
	if len(entries) > 0 {
		printTable(entries)
	}
	return exitCode
}

// filterPseudo removes filesystems with 0 total blocks per R1.1.
func filterPseudo(entries []fsInfo) []fsInfo {
	result := make([]fsInfo, 0, len(entries))
	for _, e := range entries {
		if e.TotalBlocks > 0 {
			result = append(result, e)
		}
	}
	return result
}

// R1.3: Use% = ceiling((total - available) * 100 / total).
// Returns "-" when total is 0.
func computeUsePercent(total, avail uint64) string {
	if total == 0 {
		return "-"
	}
	if avail >= total {
		return "0%"
	}
	used := total - avail
	pct := (used*100 + total - 1) / total
	return fmt.Sprintf("%d%%", pct)
}

// to1K converts block counts to 1024-byte units per R1.3.
func to1K(blocks, blockSize uint64) uint64 {
	return blocks * blockSize / 1024
}

// formatRow produces column values for one filesystem entry.
func formatRow(e fsInfo) []string {
	kbTotal := to1K(e.TotalBlocks, e.BlockSize)
	kbFree := to1K(e.FreeBlocks, e.BlockSize)
	kbAvail := to1K(e.AvailBlocks, e.BlockSize)
	kbUsed := uint64(0)
	if kbTotal > kbFree {
		kbUsed = kbTotal - kbFree
	}
	return []string{
		e.Device,
		fmt.Sprintf("%d", kbTotal),
		fmt.Sprintf("%d", kbUsed),
		fmt.Sprintf("%d", kbAvail),
		computeUsePercent(e.TotalBlocks, e.AvailBlocks),
		e.MountPoint,
	}
}

// computeWidths returns per-column maximum widths across header and rows.
// R1.5: column widths are per-column maxima of entry lengths.
func computeWidths(header []string, rows [][]string) []int {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, f := range row {
			if len(f) > widths[i] {
				widths[i] = len(f)
			}
		}
	}
	return widths
}

// isLeftAligned returns true for Filesystem (col 0) and Mounted on (last col).
func isLeftAligned(col, total int) bool {
	return col == 0 || col == total-1
}

// printRow writes one aligned row to stdout.
func printRow(fields []string, widths []int) {
	parts := make([]string, len(fields))
	for i, f := range fields {
		if isLeftAligned(i, len(fields)) {
			parts[i] = format.PadRight(f, widths[i])
		} else {
			parts[i] = format.PadLeft(f, widths[i])
		}
	}
	last := len(parts) - 1
	parts[last] = strings.TrimRight(parts[last], " ")
	fmt.Println(strings.Join(parts, " "))
}

// printTable formats and prints the filesystem table per R1.2 and R1.5.
func printTable(entries []fsInfo) {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = formatRow(e)
	}
	widths := computeWidths(defaultHeader, rows)
	printRow(defaultHeader, widths)
	for _, row := range rows {
		printRow(row, widths)
	}
}
