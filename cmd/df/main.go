// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements df: report filesystem disk space usage.
// Implements prd106-df R1.1-R1.5, R2.1-R2.3, R3.5, R3.6, R4.1-R4.3.
//
// TODO: prd106-df R2.1 task requested -B (--block-size=SIZE) but this
// conflicts with PRD non_goals which excludes --block-size=SIZE. Skipped per E6.
package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sizeMode selects how sizes are displayed.
type sizeMode int

const (
	sizeModeBlocks sizeMode = iota // default 1K-blocks
	sizeModeHuman                  // -h: powers of 1024
	sizeModeSI                     // -H: powers of 1000
)

// options holds parsed command-line flags.
type options struct {
	sizeMode     sizeMode
	includeTypes []string
	excludeTypes []string
	files        []string
}

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
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries, exitCode := collectEntries(opts.files)
	entries = filterByType(entries, opts.includeTypes, opts.excludeTypes)
	printFormatted(entries, opts)
	return exitCode
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (*options, error) {
	opts := &options{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			opts.files = append(opts.files, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(arg, args, &i, opts); err != nil {
				return nil, err
			}
			continue
		}
		if err := parseShortFlags(arg[1:], args, &i, opts); err != nil {
			return nil, err
		}
	}
	return opts, nil
}

// parseLongFlag handles a single --flag or --flag=value argument.
func parseLongFlag(arg string, args []string, idx *int, opts *options) error {
	name, val, hasVal := splitLongFlag(arg)
	switch name {
	case "--human-readable":
		opts.sizeMode = sizeModeHuman
	case "--si":
		opts.sizeMode = sizeModeSI
	case "--type":
		v, err := longFlagValue(val, hasVal, args, idx)
		if err != nil {
			return fmt.Errorf("option '--type' requires an argument")
		}
		opts.includeTypes = append(opts.includeTypes, v)
	case "--exclude-type":
		v, err := longFlagValue(val, hasVal, args, idx)
		if err != nil {
			return fmt.Errorf("option '--exclude-type' requires an argument")
		}
		opts.excludeTypes = append(opts.excludeTypes, v)
	case "--no-sync":
		// no-op: this is the default behavior
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// parseShortFlags processes combined short flags like -ht.
// R2.3: -h and -H are mutually exclusive; last one wins.
func parseShortFlags(flags string, args []string, idx *int, opts *options) error {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'h':
			opts.sizeMode = sizeModeHuman
		case 'H':
			opts.sizeMode = sizeModeSI
		case 'k':
			// no-op: 1K-blocks is the default
		case 't':
			val, err := flagValue(flags[j+1:], args, idx)
			if err != nil {
				return fmt.Errorf("option requires an argument -- 't'")
			}
			opts.includeTypes = append(opts.includeTypes, val)
			return nil
		case 'x':
			val, err := flagValue(flags[j+1:], args, idx)
			if err != nil {
				return fmt.Errorf("option requires an argument -- 'x'")
			}
			opts.excludeTypes = append(opts.excludeTypes, val)
			return nil
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return nil
}

// splitLongFlag splits --name=value into parts.
func splitLongFlag(arg string) (name, val string, hasVal bool) {
	name, val, hasVal = strings.Cut(arg, "=")
	return name, val, hasVal
}

// flagValue returns the value for a short flag that takes an argument.
// Uses the remainder of the current flag string or the next argument.
func flagValue(rest string, args []string, idx *int) (string, error) {
	if len(rest) > 0 {
		return rest, nil
	}
	if *idx+1 >= len(args) {
		return "", fmt.Errorf("missing argument")
	}
	*idx++
	return args[*idx], nil
}

// longFlagValue returns the value for a long flag that takes an argument.
func longFlagValue(val string, hasVal bool, args []string, idx *int) (string, error) {
	if hasVal {
		return val, nil
	}
	if *idx+1 >= len(args) {
		return "", fmt.Errorf("missing argument")
	}
	*idx++
	return args[*idx], nil
}

// collectEntries gathers filesystem entries from args or all mounts.
func collectEntries(files []string) ([]fsEntry, int) {
	if len(files) == 0 {
		return collectAllFilesystems()
	}
	return collectFileArgs(files)
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
func collectFileArgs(files []string) ([]fsEntry, int) {
	var entries []fsEntry
	exitCode := 0
	for _, path := range files {
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

// filterByType applies -t and -x type filters.
// R3.5: -t inclusion applied first.
// R3.6: -x exclusion applied second.
func filterByType(entries []fsEntry, include, exclude []string) []fsEntry {
	if len(include) == 0 && len(exclude) == 0 {
		return entries
	}
	result := make([]fsEntry, 0, len(entries))
	for _, e := range entries {
		if matchesTypeFilter(e.fsType, include, exclude) {
			result = append(result, e)
		}
	}
	return result
}

// matchesTypeFilter checks if a filesystem type passes include/exclude filters.
func matchesTypeFilter(fsType string, include, exclude []string) bool {
	if len(include) > 0 && !containsStr(include, fsType) {
		return false
	}
	return !containsStr(exclude, fsType)
}

// containsStr returns true if s is in the slice ss.
func containsStr(ss []string, s string) bool {
	return slices.Contains(ss, s)
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

// printFormatted outputs df with aligned columns in the selected size mode.
func printFormatted(entries []fsEntry, opts *options) {
	headers := buildHeaders(opts.sizeMode)
	rows := buildRows(entries, opts.sizeMode)
	widths := computeColumnWidths(headers, rows)
	printAlignedRow(headers, widths)
	for _, row := range rows {
		printAlignedRow(row, widths)
	}
}

// buildHeaders returns column headers for the current size mode.
// R2.1/R2.2: column header changes from "1K-blocks" to "Size" for -h/-H.
func buildHeaders(mode sizeMode) []string {
	sizeHeader := "1K-blocks"
	if mode != sizeModeBlocks {
		sizeHeader = "Size"
	}
	return []string{"Filesystem", sizeHeader, "Used", "Available", "Use%", "Mounted on"}
}

// buildRows converts filesystem entries to formatted string rows.
func buildRows(entries []fsEntry, mode sizeMode) [][]string {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			e.source,
			formatSize(e.blocks1K, mode),
			formatSize(e.used, mode),
			formatSize(e.available, mode),
			computeUsePct(e.used, e.available),
			e.mountedOn,
		}
	}
	return rows
}

// formatSize renders a 1K-block count in the selected display mode.
// R2.1: -h uses pkg/format.HumanSize with Binary=true (D1).
// R2.2: -H uses pkg/format.HumanSize with Binary=false.
func formatSize(blocks1K int64, mode sizeMode) string {
	switch mode {
	case sizeModeHuman:
		return format.HumanSize(blocks1K*1024, format.HumanSizeOpts{Binary: true})
	case sizeModeSI:
		return format.HumanSize(blocks1K*1024, format.HumanSizeOpts{Binary: false})
	default:
		return fmt.Sprintf("%d", blocks1K)
	}
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
// Columns 1-4 (sizes and Use%) are right-aligned.
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
