// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd106-df R1.1–R1.5, R2.1–R2.3 -- df core filesystem queries
// and output formatting options (human-readable and SI units).

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

// sizeMode indicates how sizes are displayed.
type sizeMode int

const (
	sizeDefault sizeMode = iota // R1.1: 1K-blocks
	sizeHuman                   // R2.1: -h binary 1024-based
	sizeSI                      // R2.2: -H SI 1000-based
)

// options holds parsed command-line flags.
type options struct {
	mode sizeMode
}

// R1.2: column headers matching GNU df exactly.
var defaultHeader = []string{
	"Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on",
}

// humanHeader replaces "1K-blocks" with "Size" for -h and -H per R2.1/R2.2.
var humanHeader = []string{
	"Filesystem", "Size", "Used", "Available", "Use%", "Mounted on",
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		return listAllFilesystems(opts)
	}
	return listPathFilesystems(files, opts)
}

// parseArgs extracts flags and file arguments from the command line.
// R2.3: -h and -H are mutually exclusive; last one wins.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		if err := parseFlag(arg, &opts); err != nil {
			return opts, nil, err
		}
	}
	return opts, files, nil
}

// parseFlag handles a single flag argument.
// TODO: prd106-df non_goals: -B/--block-size=SIZE is out of scope (E6).
func parseFlag(arg string, opts *options) error {
	switch arg {
	case "-h", "--human-readable":
		opts.mode = sizeHuman
	case "-H", "--si":
		opts.mode = sizeSI
	case "-k":
		// Non-goals: -k accepted, no visible effect (1K is default).
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// R1.1: list all mounted filesystems, excluding pseudo-filesystems.
func listAllFilesystems(opts options) int {
	all, err := getAllFilesystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries := filterPseudo(all)
	printTable(entries, opts)
	return 0
}

// R1.4: report the filesystem containing each FILE argument.
func listPathFilesystems(args []string, opts options) int {
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
		printTable(entries, opts)
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

// to1K converts block counts to 1024-byte units per R1.1.
func to1K(blocks, blockSize uint64) uint64 {
	return blocks * blockSize / 1024
}

// toBytes converts block counts to byte counts for human-readable formatting.
func toBytes(blocks, blockSize uint64) int64 {
	return int64(blocks * blockSize)
}

// formatRow produces column values for one filesystem entry.
func formatRow(e fsInfo, opts options) []string {
	if opts.mode == sizeDefault {
		return formatDefaultRow(e)
	}
	return formatHumanRow(e, opts)
}

// formatDefaultRow formats sizes in 1K-block units per R1.1.
func formatDefaultRow(e fsInfo) []string {
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

// formatHumanRow formats sizes using human-readable units per R2.1/R2.2.
func formatHumanRow(e fsInfo, opts options) []string {
	binary := opts.mode == sizeHuman
	hsOpts := format.HumanSizeOpts{Binary: binary}
	total := toBytes(e.TotalBlocks, e.BlockSize)
	free := toBytes(e.FreeBlocks, e.BlockSize)
	avail := toBytes(e.AvailBlocks, e.BlockSize)
	used := int64(0)
	if total > free {
		used = total - free
	}
	return []string{
		e.Device,
		format.HumanSize(total, hsOpts),
		format.HumanSize(used, hsOpts),
		format.HumanSize(avail, hsOpts),
		computeUsePercent(e.TotalBlocks, e.AvailBlocks),
		e.MountPoint,
	}
}

// selectHeader returns the appropriate header based on the size mode.
func selectHeader(opts options) []string {
	if opts.mode == sizeDefault {
		return defaultHeader
	}
	return humanHeader
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
func printTable(entries []fsInfo, opts options) {
	header := selectHeader(opts)
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = formatRow(e, opts)
	}
	widths := computeWidths(header, rows)
	printRow(header, widths)
	for _, row := range rows {
		printRow(row, widths)
	}
}
