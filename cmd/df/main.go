// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd106-df R1.1–R1.5, R2.1–R2.3, R3.1–R3.7, R4.1–R4.3 -- df core
// filesystem queries, output formatting, type display, inode display, filtering,
// column selection, error handling, and signal handling.

package main

import (
	"fmt"
	"os"
	"slices"
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

// networkFSTypes lists filesystem types considered non-local per R3.4.
var networkFSTypes = map[string]bool{
	"nfs": true, "nfs4": true, "cifs": true, "smbfs": true,
	"afs": true, "ncpfs": true, "coda": true, "gfs": true,
	"gfs2": true, "glusterfs": true, "lustre": true,
	"fuse.sshfs": true, "9p": true,
}

// validOutputFields lists the recognized --output field names per R3.7.
var validOutputFields = map[string]bool{
	"source": true, "fstype": true, "itotal": true, "iused": true,
	"iavail": true, "ipcent": true, "size": true, "used": true,
	"avail": true, "pcent": true, "file": true, "target": true,
}

// allOutputFields is the default field order when --output is given
// without a field list, per R3.7.
var allOutputFields = []string{
	"source", "fstype", "itotal", "iused", "iavail", "ipcent",
	"size", "used", "avail", "pcent", "file", "target",
}

// options holds parsed command-line flags.
type options struct {
	mode         sizeMode
	printType    bool     // R3.1: -T
	inodes       bool     // R3.2: -i
	all          bool     // R3.3: -a
	local        bool     // R3.4: -l
	includeTypes []string // R3.5: -t TYPE
	excludeTypes []string // R3.6: -x TYPE
	outputFields []string // R3.7: --output=FIELD_LIST
	hasOutput    bool     // R3.7: true when --output is given
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
	// R3.7: --output is incompatible with -i, -T, and -h/-H.
	if err := validateOutputCompat(opts); err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		return listAllFilesystems(opts)
	}
	return listPathFilesystems(files, opts)
}

// validateOutputCompat checks --output incompatibility per R3.7.
func validateOutputCompat(opts options) error {
	if !opts.hasOutput {
		return nil
	}
	if opts.inodes {
		return fmt.Errorf("options -i and --output are mutually exclusive")
	}
	if opts.printType {
		return fmt.Errorf("options -T and --output are mutually exclusive")
	}
	if opts.mode != sizeDefault {
		return fmt.Errorf("options -h/-H and --output are mutually exclusive")
	}
	return nil
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
		consumed, err := parseFlag(arg, args, i, &opts)
		if err != nil {
			return opts, nil, err
		}
		i += consumed
	}
	return opts, files, nil
}

// parseFlag handles a single flag argument. Returns extra args consumed.
func parseFlag(arg string, args []string, idx int, opts *options) (int, error) {
	// R3.5: --type=TYPE
	if strings.HasPrefix(arg, "--type=") {
		opts.includeTypes = append(opts.includeTypes, arg[len("--type="):])
		return 0, nil
	}
	// R3.6: --exclude-type=TYPE
	if strings.HasPrefix(arg, "--exclude-type=") {
		opts.excludeTypes = append(opts.excludeTypes, arg[len("--exclude-type="):])
		return 0, nil
	}
	// R3.7: --output or --output=FIELD_LIST
	if arg == "--output" || strings.HasPrefix(arg, "--output=") {
		return 0, parseOutputFlag(arg, opts)
	}
	return parseSingleFlag(arg, args, idx, opts)
}

// parseOutputFlag handles --output and --output=FIELD_LIST per R3.7.
func parseOutputFlag(arg string, opts *options) error {
	opts.hasOutput = true
	if arg == "--output" {
		opts.outputFields = allOutputFields
		return nil
	}
	fieldList := arg[len("--output="):]
	fields := strings.Split(fieldList, ",")
	for _, f := range fields {
		if !validOutputFields[f] {
			return fmt.Errorf("'%s' is not a valid field for --output", f)
		}
	}
	opts.outputFields = fields
	return nil
}

// parseSingleFlag handles simple flags and -t/-x with a following arg.
func parseSingleFlag(arg string, args []string, idx int, opts *options) (int, error) {
	switch arg {
	case "-h", "--human-readable":
		opts.mode = sizeHuman
	case "-H", "--si":
		opts.mode = sizeSI
	case "-k":
		// Non-goals: -k accepted, no visible effect (1K is default).
	case "-T", "--print-type":
		opts.printType = true
	case "-i", "--inodes":
		opts.inodes = true
	case "-a", "--all":
		opts.all = true
	case "-l", "--local":
		opts.local = true
	case "-t", "--type":
		// R3.5: -t TYPE needs next argument.
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '%s' requires an argument", arg)
		}
		opts.includeTypes = append(opts.includeTypes, args[idx+1])
		return 1, nil
	case "-x", "--exclude-type":
		// R3.6: -x TYPE needs next argument.
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '%s' requires an argument", arg)
		}
		opts.excludeTypes = append(opts.excludeTypes, args[idx+1])
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// R1.1: list all mounted filesystems, applying filters.
func listAllFilesystems(opts options) int {
	all, err := getAllFilesystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries := applyFilters(all, opts)
	printOutput(entries, opts)
	return 0
}

// applyFilters applies pseudo-fs, local, type, and exclusion filters.
func applyFilters(entries []fsInfo, opts options) []fsInfo {
	result := make([]fsInfo, 0, len(entries))
	for _, e := range entries {
		if !opts.all && e.TotalBlocks == 0 {
			continue
		}
		if opts.local && isNetworkFS(e.FSType) {
			continue
		}
		// R3.5: -t inclusion applied first.
		if len(opts.includeTypes) > 0 && !matchesType(e.FSType, opts.includeTypes) {
			continue
		}
		// R3.6: -x exclusion applied after -t inclusion.
		if matchesType(e.FSType, opts.excludeTypes) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// matchesType returns true if fsType matches any type in the list.
func matchesType(fsType string, types []string) bool {
	return slices.Contains(types, fsType)
}

// isNetworkFS returns true if the filesystem type is a network filesystem.
// R3.4: used to exclude non-local filesystems when -l is given.
func isNetworkFS(fsType string) bool {
	return networkFSTypes[strings.ToLower(fsType)]
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
		printOutput(entries, opts)
	}
	return exitCode
}

// printOutput dispatches to --output mode or standard table mode.
func printOutput(entries []fsInfo, opts options) {
	if opts.hasOutput {
		printOutputTable(entries, opts)
		return
	}
	printTable(entries, opts)
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
	if opts.inodes {
		return formatInodeRow(e, opts)
	}
	if opts.mode == sizeDefault {
		return formatDefaultRow(e, opts)
	}
	return formatHumanRow(e, opts)
}

// formatDefaultRow formats sizes in 1K-block units per R1.1.
func formatDefaultRow(e fsInfo, opts options) []string {
	kbTotal := to1K(e.TotalBlocks, e.BlockSize)
	kbFree := to1K(e.FreeBlocks, e.BlockSize)
	kbAvail := to1K(e.AvailBlocks, e.BlockSize)
	kbUsed := uint64(0)
	if kbTotal > kbFree {
		kbUsed = kbTotal - kbFree
	}
	row := []string{e.Device}
	if opts.printType {
		row = append(row, e.FSType)
	}
	row = append(row,
		fmt.Sprintf("%d", kbTotal),
		fmt.Sprintf("%d", kbUsed),
		fmt.Sprintf("%d", kbAvail),
		computeUsePercent(e.TotalBlocks, e.AvailBlocks),
		e.MountPoint,
	)
	return row
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
	row := []string{e.Device}
	if opts.printType {
		row = append(row, e.FSType)
	}
	row = append(row,
		format.HumanSize(total, hsOpts),
		format.HumanSize(used, hsOpts),
		format.HumanSize(avail, hsOpts),
		computeUsePercent(e.TotalBlocks, e.AvailBlocks),
		e.MountPoint,
	)
	return row
}

// formatInodeRow formats inode counts per R3.2.
func formatInodeRow(e fsInfo, opts options) []string {
	iUsed := uint64(0)
	if e.TotalInodes > e.FreeInodes {
		iUsed = e.TotalInodes - e.FreeInodes
	}
	row := []string{e.Device}
	if opts.printType {
		row = append(row, e.FSType)
	}
	row = append(row,
		fmt.Sprintf("%d", e.TotalInodes),
		fmt.Sprintf("%d", iUsed),
		fmt.Sprintf("%d", e.FreeInodes),
		computeUsePercent(e.TotalInodes, e.FreeInodes),
		e.MountPoint,
	)
	return row
}

// selectHeader returns the appropriate header based on options.
func selectHeader(opts options) []string {
	if opts.inodes {
		return buildInodeHeader(opts)
	}
	if opts.mode == sizeDefault {
		return buildDefaultHeader(opts)
	}
	return buildHumanHeader(opts)
}

// buildDefaultHeader returns the default column header with optional Type.
func buildDefaultHeader(opts options) []string {
	h := []string{"Filesystem"}
	if opts.printType {
		h = append(h, "Type")
	}
	return append(h, "1K-blocks", "Used", "Available", "Use%", "Mounted on")
}

// buildHumanHeader returns the human-readable header with optional Type.
func buildHumanHeader(opts options) []string {
	h := []string{"Filesystem"}
	if opts.printType {
		h = append(h, "Type")
	}
	return append(h, "Size", "Used", "Available", "Use%", "Mounted on")
}

// buildInodeHeader returns the inode header per R3.2 with optional Type.
func buildInodeHeader(opts options) []string {
	h := []string{"Filesystem"}
	if opts.printType {
		h = append(h, "Type")
	}
	return append(h, "Inodes", "IUsed", "IFree", "IUse%", "Mounted on")
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

// --- R3.7: --output column selection ---

// outputFieldHeader returns the display header for an --output field.
func outputFieldHeader(field string) string {
	headers := map[string]string{
		"source": "Filesystem", "fstype": "Type",
		"itotal": "Inodes", "iused": "IUsed",
		"iavail": "IFree", "ipcent": "IUse%",
		"size": "1K-blocks", "used": "Used",
		"avail": "Avail", "pcent": "Use%",
		"file": "File", "target": "Mounted on",
	}
	return headers[field]
}

// outputFieldValue returns the value for a single --output field.
func outputFieldValue(e fsInfo, field string) string {
	kbTotal := to1K(e.TotalBlocks, e.BlockSize)
	kbFree := to1K(e.FreeBlocks, e.BlockSize)
	kbAvail := to1K(e.AvailBlocks, e.BlockSize)
	kbUsed := uint64(0)
	if kbTotal > kbFree {
		kbUsed = kbTotal - kbFree
	}
	return outputFieldLookup(e, field, kbTotal, kbUsed, kbAvail)
}

// outputFieldLookup maps a field name to its display value.
func outputFieldLookup(e fsInfo, field string, kbTotal, kbUsed, kbAvail uint64) string {
	iUsed := uint64(0)
	if e.TotalInodes > e.FreeInodes {
		iUsed = e.TotalInodes - e.FreeInodes
	}
	switch field {
	case "source":
		return e.Device
	case "fstype":
		return e.FSType
	case "itotal":
		return fmt.Sprintf("%d", e.TotalInodes)
	case "iused":
		return fmt.Sprintf("%d", iUsed)
	case "iavail":
		return fmt.Sprintf("%d", e.FreeInodes)
	case "ipcent":
		return computeUsePercent(e.TotalInodes, e.FreeInodes)
	case "size":
		return fmt.Sprintf("%d", kbTotal)
	case "used":
		return fmt.Sprintf("%d", kbUsed)
	case "avail":
		return fmt.Sprintf("%d", kbAvail)
	case "pcent":
		return computeUsePercent(e.TotalBlocks, e.AvailBlocks)
	case "file":
		return e.MountPoint
	case "target":
		return e.MountPoint
	default:
		return ""
	}
}

// outputIsLeftAligned returns true for text fields in --output mode.
func outputIsLeftAligned(field string) bool {
	switch field {
	case "source", "fstype", "file", "target":
		return true
	default:
		return false
	}
}

// printOutputTable formats and prints --output column-selected table.
func printOutputTable(entries []fsInfo, opts options) {
	fields := opts.outputFields
	header := make([]string, len(fields))
	for i, f := range fields {
		header[i] = outputFieldHeader(f)
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		row := make([]string, len(fields))
		for j, f := range fields {
			row[j] = outputFieldValue(e, f)
		}
		rows[i] = row
	}
	widths := computeWidths(header, rows)
	printOutputRow(header, widths, fields)
	for _, row := range rows {
		printOutputRow(row, widths, fields)
	}
}

// printOutputRow writes one aligned row for --output mode.
func printOutputRow(values []string, widths []int, fields []string) {
	parts := make([]string, len(values))
	for i, v := range values {
		if outputIsLeftAligned(fields[i]) {
			parts[i] = format.PadRight(v, widths[i])
		} else {
			parts[i] = format.PadLeft(v, widths[i])
		}
	}
	last := len(parts) - 1
	parts[last] = strings.TrimRight(parts[last], " ")
	fmt.Println(strings.Join(parts, " "))
}
