// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements df: report filesystem disk space usage.
// Implements srd106-df R1.1-R1.5, R3.2, R3.7, R4.1-R4.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

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
}

// config holds parsed command-line options.
type config struct {
	inodes    bool
	hasOutput bool
	outFields []string // specific fields; nil means all when hasOutput
	paths     []string
}

// --- Column value functions ---

func valSource(e dfEntry) string { return e.mount.source }
func valTarget(e dfEntry) string { return e.mount.target }
func valFsType(e dfEntry) string { return e.mount.fsType }
func valFile(e dfEntry) string   { return e.filePath }

func valSize(e dfEntry) string {
	m := e.mount
	return fmt.Sprintf("%d", int64(m.totalBlocks)*m.blockSize/1024)
}

func valUsed(e dfEntry) string {
	m := e.mount
	return fmt.Sprintf("%d", (int64(m.totalBlocks)-int64(m.freeBlocks))*m.blockSize/1024)
}

func valAvail(e dfEntry) string {
	m := e.mount
	return fmt.Sprintf("%d", int64(m.availBlocks)*m.blockSize/1024)
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

// --- Column definitions ---

// defaultCols is the column set for default block-usage mode (R1.1, R1.2).
var defaultCols = []colDef{
	{"Filesystem", false, valSource},
	{"1K-blocks", true, valSize},
	{"Used", true, valUsed},
	{"Available", true, valAvail},
	{"Use%", true, valPcent},
	{"Mounted on", false, valTarget},
}

// inodeCols is the column set for inode mode (R3.2).
var inodeCols = []colDef{
	{"Filesystem", false, valSource},
	{"Inodes", true, valItotal},
	{"IUsed", true, valIused},
	{"IFree", true, valIavail},
	{"IUse%", true, valIpcent},
	{"Mounted on", false, valTarget},
}

// outputFieldMap maps --output field names to column definitions (R3.7).
var outputFieldMap = map[string]colDef{
	"source": {"Filesystem", false, valSource},
	"fstype": {"Type", false, valFsType},
	"itotal": {"Inodes", true, valItotal},
	"iused":  {"IUsed", true, valIused},
	"iavail": {"IFree", true, valIavail},
	"ipcent": {"IUse%", true, valIpcent},
	"size":   {"1K-blocks", true, valSize},
	"used":   {"Used", true, valUsed},
	"avail":  {"Avail", true, valAvail},
	"pcent":  {"Use%", true, valPcent},
	"file":   {"File", false, valFile},
	"target": {"Mounted on", false, valTarget},
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

// TODO: --total is listed in srd106-df non_goals. Task references it but
// it conflicts with the SRD non_goals list. Skipped per constitution E6.

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
	for i := 0; i < len(args); i++ {
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

// validateConfig checks for incompatible flag combinations (R3.7).
func validateConfig(cfg config) error {
	if cfg.hasOutput && cfg.inodes {
		return fmt.Errorf("-i and --output are mutually exclusive")
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
	if cfg.inodes {
		return inodeCols
	}
	return defaultCols
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
func computeWidths(cols []colDef, rows [][]string) []int {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c.header)
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
