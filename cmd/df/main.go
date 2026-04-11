// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements df: report filesystem disk space usage.
// Implements srd106-df R1.1-R1.5, R2.1-R2.3, R3.1, R3.2, R3.7, R4.1-R4.3.
package main

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sizeMode controls how size columns are formatted.
// R2.1: sizeHuman uses binary units (K, M, G).
// R2.2: sizeSI uses SI units (k, M, G).
type sizeMode int

const (
	sizeDefault sizeMode = iota
	sizeHuman
	sizeSI
)

// humanMinWidth is the minimum column width GNU df applies to
// human-readable size columns (Size, Used, Avail).
const humanMinWidth = 5

// mountInfo holds filesystem data from the OS.
type mountInfo struct {
	source      string
	target      string
	fsType      string
	totalBlocks uint64
	freeBlocks  uint64
	availBlocks uint64
	blockSize   int64
	totalInodes uint64
	freeInodes  uint64
}

// dfEntry wraps a mount with the FILE argument that selected it.
type dfEntry struct {
	mount    mountInfo
	filePath string
}

// colDef defines a single output column.
type colDef struct {
	header     string
	rightAlign bool
	getValue   func(dfEntry) string
	minWidth   int // minimum column width; 0 = use header/data max
}

// config holds parsed command-line options.
type config struct {
	inodes    bool
	printType bool
	sizeMode  sizeMode
	hasOutput bool
	outFields []string // specific fields; nil means all when hasOutput
	paths     []string
}

// --- Column value functions ---

func valSource(e dfEntry) string { return e.mount.source }
func valTarget(e dfEntry) string { return e.mount.target }
func valFsType(e dfEntry) string { return e.mount.fsType }
func valFile(e dfEntry) string   { return e.filePath }

// totalBytes returns the total size in bytes for a filesystem.
func totalBytes(e dfEntry) int64 {
	return int64(e.mount.totalBlocks) * e.mount.blockSize
}

// usedBytes returns the used size in bytes for a filesystem.
func usedBytes(e dfEntry) int64 {
	m := e.mount
	used := max(int64(m.totalBlocks)-int64(m.freeBlocks), 0)
	return used * m.blockSize
}

// availBytes returns the available size in bytes for a filesystem.
func availBytes(e dfEntry) int64 {
	return int64(e.mount.availBlocks) * e.mount.blockSize
}

// valSize returns the total size in 1K-blocks.
func valSize(e dfEntry) string {
	return fmt.Sprintf("%d", totalBytes(e)/1024)
}

// valUsed returns the used size in 1K-blocks.
func valUsed(e dfEntry) string {
	return fmt.Sprintf("%d", usedBytes(e)/1024)
}

// valAvail returns the available size in 1K-blocks.
func valAvail(e dfEntry) string {
	return fmt.Sprintf("%d", availBytes(e)/1024)
}

func valPcent(e dfEntry) string {
	m := e.mount
	return computeUsePct(m.totalBlocks, m.freeBlocks, m.availBlocks)
}

func valItotal(e dfEntry) string {
	return fmt.Sprintf("%d", e.mount.totalInodes)
}

func valIused(e dfEntry) string {
	m := e.mount
	var used uint64
	if m.totalInodes > m.freeInodes {
		used = m.totalInodes - m.freeInodes
	}
	return fmt.Sprintf("%d", used)
}

func valIavail(e dfEntry) string {
	return fmt.Sprintf("%d", e.mount.freeInodes)
}

func valIpcent(e dfEntry) string {
	m := e.mount
	return computeUsePct(m.totalInodes, m.freeInodes, m.freeInodes)
}

// --- Size formatting ---

// formatSizeVal formats a byte count according to the active sizeMode.
// R2.1: sizeHuman uses 1024-based ceiling (K, M, G).
// R2.2: sizeSI uses 1000-based ceiling (k, M, G).
func formatSizeVal(b int64, sm sizeMode) string {
	switch sm {
	case sizeHuman:
		return gnuHumanSize(b, true)
	case sizeSI:
		return gnuHumanSize(b, false)
	default:
		return fmt.Sprintf("%d", b/1024)
	}
}

// gnuHumanSize formats bytes using GNU coreutils human_readable() conventions
// with ceiling rounding. binary=true uses 1024-base, binary=false uses 1000-base.
func gnuHumanSize(bytes int64, binary bool) string {
	if bytes == 0 {
		return "0"
	}
	base := 1000.0
	suffixes := []string{"", "k", "M", "G", "T", "P", "E"}
	if binary {
		base = 1024.0
		suffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	}
	return formatCeilSize(float64(bytes), base, suffixes)
}

// formatCeilSize formats a value with the given base and suffixes,
// using ceiling rounding matching GNU coreutils behavior.
func formatCeilSize(val float64, base float64, suffixes []string) string {
	idx := 0
	for idx+1 < len(suffixes) && val >= base {
		val /= base
		idx++
	}
	if suffixes[idx] == "" {
		return fmt.Sprintf("%.0f", math.Ceil(val))
	}
	tenths := math.Ceil(val*10) / 10
	if tenths >= 10 {
		return fmt.Sprintf("%.0f%s", math.Ceil(val), suffixes[idx])
	}
	return fmt.Sprintf("%.1f%s", tenths, suffixes[idx])
}

// makeSizeFunc returns a value function that formats bytes using sizeMode.
func makeSizeFunc(sm sizeMode, fn func(dfEntry) int64) func(dfEntry) string {
	return func(e dfEntry) string {
		return formatSizeVal(fn(e), sm)
	}
}

// --- Column definitions ---

// buildDefaultCols creates the default column set for the given sizeMode.
// R2.1/R2.2: header changes from "1K-blocks" to "Size" in human-readable modes.
func buildDefaultCols(sm sizeMode) []colDef {
	sizeHeader := "1K-blocks"
	availHeader := "Available"
	minW := 0
	if sm != sizeDefault {
		sizeHeader = "Size"
		availHeader = "Avail"
		minW = humanMinWidth
	}
	return []colDef{
		{"Filesystem", false, valSource, 0},
		{sizeHeader, true, makeSizeFunc(sm, totalBytes), minW},
		{"Used", true, makeSizeFunc(sm, usedBytes), minW},
		{availHeader, true, makeSizeFunc(sm, availBytes), minW},
		{"Use%", true, valPcent, 0},
		{"Mounted on", false, valTarget, 0},
	}
}

// insertTypeCol inserts a Type column after Filesystem.
// R3.1: -T adds a Type column showing filesystem type.
func insertTypeCol(cols []colDef) []colDef {
	typeCol := colDef{"Type", false, valFsType, 0}
	result := make([]colDef, 0, len(cols)+1)
	result = append(result, cols[0], typeCol)
	result = append(result, cols[1:]...)
	return result
}

// inodeCols is the column set for inode mode (R3.2).
var inodeCols = []colDef{
	{"Filesystem", false, valSource, 0},
	{"Inodes", true, valItotal, 0},
	{"IUsed", true, valIused, 0},
	{"IFree", true, valIavail, 0},
	{"IUse%", true, valIpcent, 0},
	{"Mounted on", false, valTarget, 0},
}

// outputFieldMap maps --output field names to column definitions (R3.7).
var outputFieldMap = map[string]colDef{
	"source": {"Filesystem", false, valSource, 0},
	"fstype": {"Type", false, valFsType, 0},
	"itotal": {"Inodes", true, valItotal, 0},
	"iused":  {"IUsed", true, valIused, 0},
	"iavail": {"IFree", true, valIavail, 0},
	"ipcent": {"IUse%", true, valIpcent, 0},
	"size":   {"1K-blocks", true, valSize, 0},
	"used":   {"Used", true, valUsed, 0},
	"avail":  {"Avail", true, valAvail, 0},
	"pcent":  {"Use%", true, valPcent, 0},
	"file":   {"File", false, valFile, 0},
	"target": {"Mounted on", false, valTarget, 0},
}

// outputAllOrder defines the canonical field order for --output without a field list (R3.7).
var outputAllOrder = []string{
	"source", "fstype", "itotal", "iused", "iavail", "ipcent",
	"size", "used", "avail", "pcent", "file", "target",
}

// dummyTypes lists filesystem types that GNU df excludes by default.
var dummyTypes = map[string]bool{
	"autofs": true, "devfs": true, "fdescfs": true,
	"linsysfs": true, "linprocfs": true, "none": true,
	"nullfs": true, "procfs": true,
}

// TODO: -B/--block-size=SIZE is listed in srd106-df non_goals.
// Task R3 references it but it conflicts with the SRD non_goals list.
// Skipped per constitution E6.

// R4.3: install SIGPIPE handler at startup.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run parses arguments, collects filesystem data, and prints output.
// R4.1: returns 0 on success. R4.2: returns 1 on any error.
func run() int {
	cfg := parseConfig(os.Args[1:])
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return 1
	}
	entries, hasError := collectEntries(cfg)
	cols := selectColumns(cfg)
	if len(entries) > 0 {
		printTable(cols, entries)
	}
	if hasError {
		return 1
	}
	return 0
}

// parseConfig extracts options and FILE paths from command-line arguments.
func parseConfig(args []string) config {
	var cfg config
	stopFlags := false
	for i := range len(args) {
		arg := args[i]
		if stopFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.paths = append(cfg.paths, arg)
			continue
		}
		if arg == "--" {
			stopFlags = true
			continue
		}
		handleFlag(&cfg, arg)
	}
	return cfg
}

// handleFlag processes a single flag argument.
func handleFlag(cfg *config, arg string) {
	switch {
	case arg == "-i" || arg == "--inodes":
		cfg.inodes = true
	case arg == "-h" || arg == "--human-readable":
		// R2.1: binary human-readable. R2.3: last flag wins.
		cfg.sizeMode = sizeHuman
	case arg == "-H" || arg == "--si":
		// R2.2: SI units. R2.3: last flag wins.
		cfg.sizeMode = sizeSI
	case arg == "-T" || arg == "--print-type":
		// R3.1: insert Type column.
		cfg.printType = true
	case arg == "--output":
		cfg.hasOutput = true
	case strings.HasPrefix(arg, "--output="):
		cfg.hasOutput = true
		fieldStr := strings.TrimPrefix(arg, "--output=")
		if fieldStr != "" {
			cfg.outFields = strings.Split(fieldStr, ",")
		}
	case isNoOpFlag(arg):
		// -k and --no-sync are accepted but ignored.
	default:
		cfg.paths = append(cfg.paths, arg)
	}
}

// isNoOpFlag returns true for flags accepted but without visible effect.
func isNoOpFlag(arg string) bool {
	return arg == "-k" || arg == "--no-sync"
}

// validateConfig checks for incompatible flag combinations.
// R3.7: --output is incompatible with -i, -T, and -h/-H.
func validateConfig(cfg config) error {
	if !cfg.hasOutput {
		return nil
	}
	if cfg.inodes {
		return fmt.Errorf("-i and --output are mutually exclusive")
	}
	if cfg.printType {
		return fmt.Errorf("-T and --output are mutually exclusive")
	}
	if cfg.sizeMode != sizeDefault {
		return fmt.Errorf("-h/-H and --output are mutually exclusive")
	}
	return validateOutputFields(cfg)
}

// validateOutputFields checks that all --output fields are recognized.
func validateOutputFields(cfg config) error {
	if !cfg.hasOutput || cfg.outFields == nil {
		return nil
	}
	for _, f := range cfg.outFields {
		if _, ok := outputFieldMap[f]; !ok {
			return fmt.Errorf("option --output: invalid field '%s'", f)
		}
	}
	return nil
}

// selectColumns returns the column definitions for the active mode.
func selectColumns(cfg config) []colDef {
	if cfg.hasOutput {
		return selectOutputColumns(cfg.outFields)
	}
	var cols []colDef
	if cfg.inodes {
		cols = inodeCols
	} else {
		cols = buildDefaultCols(cfg.sizeMode)
	}
	if cfg.printType {
		cols = insertTypeCol(cols)
	}
	return cols
}

// selectOutputColumns builds columns from the --output field list.
// nil fields means all fields in canonical order.
func selectOutputColumns(fields []string) []colDef {
	if fields == nil {
		fields = outputAllOrder
	}
	cols := make([]colDef, len(fields))
	for i, f := range fields {
		cols[i] = outputFieldMap[f]
	}
	return cols
}

// collectEntries gathers filesystem entries for the given config.
// R1.1: no paths means all mounted filesystems.
// R1.4: paths means report filesystem containing each file.
func collectEntries(cfg config) ([]dfEntry, bool) {
	if len(cfg.paths) == 0 {
		return collectAllMounts()
	}
	return collectForPaths(cfg.paths)
}

// collectAllMounts returns entries for all non-pseudo mounted filesystems.
// R1.1: excludes pseudo-filesystems (0 total blocks) and dummy types.
func collectAllMounts() ([]dfEntry, bool) {
	mounts, err := getMounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "df: %v\n", err)
		return nil, true
	}
	var entries []dfEntry
	seenDev := make(map[uint64]bool)
	for _, m := range mounts {
		if skipMount(m, seenDev) {
			continue
		}
		entries = append(entries, dfEntry{mount: m, filePath: m.source})
	}
	return entries, false
}

// skipMount returns true if a mount entry should be excluded from output.
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

// collectForPaths returns entries for filesystems containing the given files.
// R1.4: reports errors for non-existent files and continues.
func collectForPaths(paths []string) ([]dfEntry, bool) {
	var entries []dfEntry
	hasError := false
	for _, p := range paths {
		m, err := getFilesystemInfo(p)
		if err != nil {
			reportError(p, err)
			hasError = true
			continue
		}
		entries = append(entries, dfEntry{mount: *m, filePath: p})
	}
	return entries, hasError
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
// R1.2: header and alignment matching GNU df. R1.5: column widths.
func printTable(cols []colDef, entries []dfEntry) {
	rows := buildRows(cols, entries)
	widths := computeWidths(cols, rows)
	printRow(cols, extractHeaders(cols), widths)
	for _, r := range rows {
		printRow(cols, r, widths)
	}
}

// buildRows converts entries to string rows using column definitions.
func buildRows(cols []colDef, entries []dfEntry) [][]string {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = c.getValue(e)
		}
		rows[i] = row
	}
	return rows
}

// extractHeaders returns the header strings from column definitions.
func extractHeaders(cols []colDef) []string {
	hdrs := make([]string, len(cols))
	for i, c := range cols {
		hdrs[i] = c.header
	}
	return hdrs
}

// computeWidths returns the per-column max width including headers.
// R1.5: column widths are per-column maxima across all rows.
// Respects colDef.minWidth for human-readable size columns.
func computeWidths(cols []colDef, rows [][]string) []int {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = max(len(c.header), c.minWidth)
	}
	for _, r := range rows {
		for i, v := range r {
			if len(v) > widths[i] {
				widths[i] = len(v)
			}
		}
	}
	return widths
}

// printRow prints one row with proper alignment.
// R1.2: numeric columns right-aligned, text columns left-aligned.
func printRow(cols []colDef, vals []string, widths []int) {
	var buf strings.Builder
	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(' ')
		}
		if i == len(vals)-1 {
			buf.WriteString(v)
		} else if cols[i].rightAlign {
			fmt.Fprintf(&buf, "%*s", widths[i], v)
		} else {
			fmt.Fprintf(&buf, "%-*s", widths[i], v)
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
