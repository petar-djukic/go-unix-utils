// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements df: report filesystem disk space usage.
// Implements prd106-df R1.1-R1.5, R2.1-R2.3, R3.1-R3.6, R4.1-R4.3.
//
// TODO: prd106-df R2.1 task requested -B (--block-size=SIZE) but this
// conflicts with PRD non_goals which excludes --block-size=SIZE. Skipped per E6.
//
// TODO: Task requested -P (--portability) but prd106-df non_goals explicitly
// excludes -P (POSIX output format). Skipped per E6.
//
// TODO: Task requested --total (grand total row) but prd106-df non_goals
// explicitly excludes --total. Skipped per E6.
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
	showType     bool
	showInodes   bool
	showAll      bool
	localOnly    bool
	includeTypes []string
	excludeTypes []string
	files        []string
}

// fsEntry holds filesystem statistics for a single mount point.
type fsEntry struct {
	source      string
	fsType      string
	blocks1K    int64
	used        int64
	available   int64
	inodesTotal int64
	inodesUsed  int64
	inodesFree  int64
	mountedOn   string
}

// networkFSTypes identifies network filesystem types for -l filtering.
// R3.4: -l excludes network filesystems.
var networkFSTypes = map[string]bool{
	"nfs":    true,
	"nfs4":   true,
	"smbfs":  true,
	"cifs":   true,
	"afs":    true,
	"ncpfs":  true,
	"afpfs":  true,
	"webdav": true,
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
	entries, exitCode := collectEntries(opts)
	entries = applyFilters(entries, opts)
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
	case "--print-type":
		opts.showType = true
	case "--inodes":
		opts.showInodes = true
	case "--all":
		opts.showAll = true
	case "--local":
		opts.localOnly = true
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

// parseShortFlags processes combined short flags like -hTi.
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
		case 'T':
			opts.showType = true
		case 'i':
			opts.showInodes = true
		case 'a':
			opts.showAll = true
		case 'l':
			opts.localOnly = true
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
func collectEntries(opts *options) ([]fsEntry, int) {
	if len(opts.files) == 0 {
		return collectAllFilesystems(opts.showAll)
	}
	return collectFileArgs(opts.files)
}

// collectAllFilesystems returns all mounted filesystems.
// R1.1: exclude pseudo-filesystems unless -a is given.
// R3.3: -a includes pseudo-filesystems.
func collectAllFilesystems(showAll bool) ([]fsEntry, int) {
	entries, err := enumerateFilesystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return nil, 1
	}
	if !showAll {
		entries = filterPseudo(entries)
	}
	return entries, 0
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

// applyFilters applies -l and -t/-x filters to entries.
func applyFilters(entries []fsEntry, opts *options) []fsEntry {
	if opts.localOnly {
		entries = filterLocal(entries)
	}
	entries = filterByType(entries, opts.includeTypes, opts.excludeTypes)
	return entries
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

// filterLocal removes network filesystems.
// R3.4: -l restricts output to local filesystems only.
func filterLocal(entries []fsEntry) []fsEntry {
	result := make([]fsEntry, 0, len(entries))
	for _, e := range entries {
		if !networkFSTypes[e.fsType] {
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
	if len(include) > 0 && !slices.Contains(include, fsType) {
		return false
	}
	return !slices.Contains(exclude, fsType)
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

// printFormatted outputs df with aligned columns in the selected mode.
func printFormatted(entries []fsEntry, opts *options) {
	headers := buildHeaders(opts)
	rows := buildRows(entries, opts)
	widths := computeColumnWidths(headers, rows)
	rightAlign := buildAlignments(headers)
	printAlignedRow(headers, widths, rightAlign)
	for _, row := range rows {
		printAlignedRow(row, widths, rightAlign)
	}
}

// buildHeaders returns column headers based on current options.
// R2.1/R2.2: header changes from "1K-blocks" to "Size" for -h/-H.
// R3.1: -T inserts "Type" column after "Filesystem".
// R3.2: -i replaces block columns with inode columns.
func buildHeaders(opts *options) []string {
	var h []string
	h = append(h, "Filesystem")
	if opts.showType {
		h = append(h, "Type")
	}
	if opts.showInodes {
		h = append(h, "Inodes", "IUsed", "IFree", "IUse%")
	} else {
		sizeHeader := "1K-blocks"
		if opts.sizeMode != sizeModeBlocks {
			sizeHeader = "Size"
		}
		h = append(h, sizeHeader, "Used", "Available", "Use%")
	}
	h = append(h, "Mounted on")
	return h
}

// buildAlignments returns right-alignment flags for each column.
// Filesystem, Type, and Mounted on are left-aligned; numeric columns right-aligned.
func buildAlignments(headers []string) []bool {
	ra := make([]bool, len(headers))
	for i, h := range headers {
		switch h {
		case "Filesystem", "Type", "Mounted on":
			ra[i] = false
		default:
			ra[i] = true
		}
	}
	return ra
}

// buildRows converts filesystem entries to formatted string rows.
func buildRows(entries []fsEntry, opts *options) [][]string {
	rows := make([][]string, len(entries))
	for i := range entries {
		rows[i] = buildRow(&entries[i], opts)
	}
	return rows
}

// buildRow formats a single fsEntry according to options.
// R3.1: includes fsType when showType is set.
// R3.2: shows inode columns instead of block columns when showInodes is set.
func buildRow(e *fsEntry, opts *options) []string {
	var row []string
	row = append(row, e.source)
	if opts.showType {
		row = append(row, e.fsType)
	}
	if opts.showInodes {
		row = append(row,
			fmt.Sprintf("%d", e.inodesTotal),
			fmt.Sprintf("%d", e.inodesUsed),
			fmt.Sprintf("%d", e.inodesFree),
			computeUsePct(e.inodesUsed, e.inodesFree),
		)
	} else {
		row = append(row,
			formatSize(e.blocks1K, opts.sizeMode),
			formatSize(e.used, opts.sizeMode),
			formatSize(e.available, opts.sizeMode),
			computeUsePct(e.used, e.available),
		)
	}
	row = append(row, e.mountedOn)
	return row
}

// formatSize renders a 1K-block count in the selected display mode.
// R2.1: -h uses pkg/format.HumanSize with Binary=true.
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
// Right-aligned columns use right padding; left-aligned use left padding.
// The last column has no trailing padding.
func printAlignedRow(cells []string, widths []int, rightAlign []bool) {
	parts := make([]string, len(cells))
	lastIdx := len(cells) - 1
	for i, cell := range cells {
		switch {
		case rightAlign[i]:
			parts[i] = fmt.Sprintf("%*s", widths[i], cell)
		case i == lastIdx:
			parts[i] = cell
		default:
			parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		}
	}
	fmt.Println(strings.Join(parts, " "))
}
