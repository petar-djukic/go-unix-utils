// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls implements prd008-ls: file listing with multi-column, single-column,
// and long-format output modes, filtering, sorting, color, classification,
// human-readable sizes, and recursive listing.
package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputFormat selects the output layout mode.
// R1.1, R1.5, R1.6, R1.11, R1.13: format flags -1, -l, -C, -x.
type outputFormat int

const (
	formatDefault outputFormat = iota
	formatSingle               // -1
	formatLong                 // -l
	formatColumns              // -C
	formatAcross               // -x
)

// filterMode selects which entries are visible.
// R1.4, R2.1, R2.2, R2.4: filter flags -a, -A.
type filterMode int

const (
	filterNoDot     filterMode = iota // default: hide dotfiles
	filterAll                         // -a: show all including . and ..
	filterAlmostAll                   // -A: show dotfiles except . and ..
)

// sortMode selects the primary sort key.
// R2.5, R2.6, R2.8, R2.9, R2.10: sort flags -t, -S, -U, -v.
type sortMode int

const (
	sortName    sortMode = iota // default: C locale name sort
	sortTime                    // -t: newest first
	sortSize                    // -S: largest first
	sortNone                    // -U: directory order
	sortVersion                 // -v: natural version sort
)

// colorMode selects color output behavior.
// R3.1, R3.2, R3.3, R3.4: --color flag.
type colorMode int

const (
	colorAuto   colorMode = iota // --color=auto
	colorAlways                  // --color=always
	colorNever                   // --color=never
)

// lsConfig holds all parsed flag state for a single ls invocation.
// R1 through R4: aggregates all flag state.
type lsConfig struct {
	format     outputFormat
	filter     filterMode
	sortBy     sortMode
	colorOpt   colorMode
	reverse    bool // -r: reverse sort order
	dirOnly    bool // -d: list directories themselves
	humanSize  bool // -h: human-readable sizes
	classify   bool // -F: append type indicator
	recursive  bool // -R: recurse into subdirectories
	showInode  bool // -i: show inode number
	showBlocks bool // -s: show allocated blocks
	numericIDs bool // -n: numeric UID/GID (implies long)
	termWidth  int  // terminal width for column layout
}

// lsEntry holds metadata for a single directory entry.
// R1.6, R1.7, R2.5, R2.6, R2.11, R2.12: entry metadata.
type lsEntry struct {
	name  string
	path  string
	info  *sys.FileInfo
	link  string // symlink target (R1.10)
	isDir bool
}

// defaultTermWidth is used when stdout is not a TTY with -C.
// R1.11: default 80 columns for non-TTY with -C.
const defaultTermWidth = 80

// exitOK, exitMinor, exitSerious match GNU ls exit codes.
// R4.1, R4.2, R4.3: exit code constants.
const (
	exitOK      = 0
	exitMinor   = 1
	exitSerious = 2
)

// sixMonths approximates six months for mtime display.
// R1.9: threshold for recent vs old time format.
const sixMonths = 6 * 30 * 24 * time.Hour

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses flags, executes the listing, and returns the exit code.
func run(args []string) int {
	cfg, paths, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: %s\n", err)
		return exitSerious
	}
	applyColorConfig(cfg)
	installResizeHandler(cfg)
	return listPaths(cfg, paths)
}

// parseFlags parses command-line arguments into an lsConfig and
// remaining path arguments.
// R1 through R4: flag parsing with last-flag-wins semantics.
func parseFlags(args []string) (*lsConfig, []string, error) {
	cfg := &lsConfig{}
	var paths []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || !strings.HasPrefix(arg, "-") || arg == "-" {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(cfg, arg[2:]); err != nil {
				return nil, nil, err
			}
			continue
		}
		if err := parseShortFlags(cfg, arg[1:]); err != nil {
			return nil, nil, err
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	resolveFormat(cfg)
	return cfg, paths, nil
}

// parseShortFlags processes a string of single-character flags.
func parseShortFlags(cfg *lsConfig, flags string) error {
	for _, ch := range flags {
		if err := applyShortFlag(cfg, ch); err != nil {
			return err
		}
	}
	return nil
}

// applyShortFlag applies a single short flag character to the config.
func applyShortFlag(cfg *lsConfig, ch rune) error {
	switch ch {
	case '1':
		cfg.format = formatSingle
	case 'l':
		cfg.format = formatLong
	case 'C':
		cfg.format = formatColumns
	case 'x':
		cfg.format = formatAcross
	case 'a':
		cfg.filter = filterAll
	case 'A':
		cfg.filter = filterAlmostAll
	case 'd':
		cfg.dirOnly = true
	case 'r':
		cfg.reverse = true
	case 't':
		cfg.sortBy = sortTime
	case 'S':
		cfg.sortBy = sortSize
	case 'U':
		cfg.sortBy = sortNone
	case 'v':
		cfg.sortBy = sortVersion
	case 'h':
		cfg.humanSize = true
	case 'F':
		cfg.classify = true
	case 'R':
		cfg.recursive = true
	case 'i':
		cfg.showInode = true
	case 's':
		cfg.showBlocks = true
	case 'n':
		cfg.numericIDs = true
		cfg.format = formatLong
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// parseLongFlag handles a single --flag argument (without the -- prefix).
func parseLongFlag(cfg *lsConfig, name string) error {
	if name == "color" || strings.HasPrefix(name, "color=") {
		return parseColorFlag(cfg, name)
	}
	return fmt.Errorf("unrecognized option '--%s'", name)
}

// parseColorFlag parses --color[=VALUE].
// R3.1: --color without a value defaults to "always".
func parseColorFlag(cfg *lsConfig, name string) error {
	if name == "color" {
		cfg.colorOpt = colorAlways
		return nil
	}
	if name == "color=auto" {
		cfg.colorOpt = colorAuto
		return nil
	}
	val := name[6:] // after "color="
	switch val {
	case "always", "yes", "force":
		cfg.colorOpt = colorAlways
	case "never", "no", "none":
		cfg.colorOpt = colorNever
	default:
		return fmt.Errorf("invalid argument '%s' for '--color'", val)
	}
	return nil
}

// resolveFormat determines terminal width and default output format.
// R1.1: TTY default is multi-column. R1.2: non-TTY default is single-column.
func resolveFormat(cfg *lsConfig) {
	isTTY := sys.IsTerminal(os.Stdout.Fd())
	if isTTY {
		w, err := sys.TerminalWidth()
		if err == nil {
			cfg.termWidth = w
		} else {
			cfg.termWidth = defaultTermWidth
		}
	} else {
		cfg.termWidth = defaultTermWidth
	}
	if cfg.format != formatDefault {
		return
	}
	if isTTY {
		cfg.format = formatColumns
	} else {
		cfg.format = formatSingle
	}
}

// applyColorConfig sets the process-global color state based on cfg.
// R3.1, R3.2, R3.3: maps colorMode to format.SetColorEnabled.
func applyColorConfig(cfg *lsConfig) {
	switch cfg.colorOpt {
	case colorAlways:
		format.SetColorEnabled(true)
	case colorNever:
		format.SetColorEnabled(false)
	case colorAuto:
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	}
}

// installResizeHandler registers a SIGWINCH callback.
// R4.5: updates cfg.termWidth on terminal resize.
func installResizeHandler(cfg *lsConfig) {
	sys.OnTerminalResize(func(width int) {
		cfg.termWidth = width
	})
}

// listPaths lists each path argument and returns the exit code.
// R4.1, R4.2: processes all paths, accumulates exit status.
func listPaths(cfg *lsConfig, paths []string) int {
	exitCode := exitOK
	var files []lsEntry
	var dirs []string
	for _, p := range paths {
		code := classifyArg(cfg, p, &files, &dirs)
		if code > exitCode {
			exitCode = code
		}
	}
	if len(files) > 0 {
		sortEntries(files, cfg)
		formatOutput(cfg, files)
	}
	showHeader := len(dirs) > 1 || len(files) > 0 || cfg.recursive
	code := listDirs(cfg, dirs, showHeader, len(files) > 0)
	if code > exitCode {
		exitCode = code
	}
	return exitCode
}

// listDirs iterates over directories and lists each one.
func listDirs(cfg *lsConfig, dirs []string, header, prefixBlank bool) int {
	exitCode := exitOK
	for i, d := range dirs {
		if prefixBlank || i > 0 {
			fmt.Println()
		}
		code := listDir(cfg, d, header)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// classifyArg stats a path and classifies it as a file or directory.
func classifyArg(
	cfg *lsConfig, path string,
	files *[]lsEntry, dirs *[]string,
) int {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n",
			path, osErrMsg(err))
		return exitSerious
	}
	if fi.Mode.IsDir() && !cfg.dirOnly {
		*dirs = append(*dirs, path)
		return exitOK
	}
	entry := lsEntry{
		name: path, path: path, info: fi,
		isDir: fi.Mode.IsDir(),
	}
	if fi.Mode&os.ModeSymlink != 0 {
		entry.link, _ = os.Readlink(path) // best-effort symlink target
	}
	*files = append(*files, entry)
	return exitOK
}

// listDir reads and lists the contents of a single directory.
// R1.1, R1.4: directory enumeration with filtering.
// R2.12, R2.13: total line printed when -s or -l is active.
func listDir(cfg *lsConfig, dirPath string, showHeader bool) int {
	if showHeader {
		fmt.Printf("%s:\n", dirPath)
	}
	entries, exitCode := readEntries(cfg, dirPath)
	if cfg.filter == filterAll {
		entries = addDotEntries(dirPath, entries)
	}
	entries = filterEntries(entries, cfg.filter)
	sortEntries(entries, cfg)
	if cfg.format == formatLong || cfg.showBlocks {
		printTotalLine(cfg, entries)
	}
	formatOutput(cfg, entries)
	// R3.11: recurse into subdirectories when -R is active
	if cfg.recursive {
		code := recurseSubdirs(cfg, entries)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// readEntries reads directory entries and stats each one.
// R1.7: uses sys.Lstat for metadata.
func readEntries(cfg *lsConfig, dirPath string) ([]lsEntry, int) {
	des, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': %s\n",
			dirPath, osErrMsg(err))
		return nil, exitSerious
	}
	return statDirEntries(des, dirPath)
}

// statDirEntries stats each directory entry and builds lsEntry values.
func statDirEntries(des []os.DirEntry, dirPath string) ([]lsEntry, int) {
	exitCode := exitOK
	entries := make([]lsEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		path := dirPath + "/" + name
		fi, err := sys.Lstat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n",
				path, osErrMsg(err))
			exitCode = exitMinor
			continue
		}
		entry := lsEntry{
			name: name, path: path, info: fi,
			isDir: fi.Mode.IsDir(),
		}
		if fi.Mode&os.ModeSymlink != 0 {
			entry.link, _ = os.Readlink(path) // best-effort symlink target
		}
		entries = append(entries, entry)
	}
	return entries, exitCode
}

// addDotEntries prepends "." and ".." entries for -a mode.
// R2.1: include . and .. when -a is given.
func addDotEntries(dirPath string, entries []lsEntry) []lsEntry {
	dots := make([]lsEntry, 0, 2+len(entries))
	for _, name := range []string{".", ".."} {
		path := dirPath + "/" + name
		fi, err := sys.Lstat(path)
		if err != nil {
			continue
		}
		dots = append(dots, lsEntry{
			name: name, path: path, info: fi,
			isDir: fi.Mode.IsDir(),
		})
	}
	return append(dots, entries...)
}

// filterEntries applies the filter mode to remove hidden entries.
// R1.4: hide dotfiles by default. R2.1, R2.2, R2.4: -a/-A overrides.
func filterEntries(entries []lsEntry, fm filterMode) []lsEntry {
	if fm == filterAll {
		return entries
	}
	result := make([]lsEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.name, ".") {
			if fm != filterAlmostAll {
				continue
			}
			if e.name == "." || e.name == ".." {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

// sortEntries sorts entries according to the configured sort mode.
// R1.3: C locale sort. R2.5-R2.10: sort modes with reverse support.
func sortEntries(entries []lsEntry, cfg *lsConfig) {
	if cfg.sortBy == sortNone {
		if cfg.reverse {
			reverseEntries(entries)
		}
		return
	}
	less := selectLessFunc(entries, cfg.sortBy)
	if cfg.reverse {
		orig := less
		less = func(i, j int) bool { return orig(j, i) }
	}
	sort.SliceStable(entries, less)
}

// selectLessFunc returns the comparison function for the given sort mode.
func selectLessFunc(entries []lsEntry, sm sortMode) func(i, j int) bool {
	switch sm {
	case sortTime:
		return func(i, j int) bool {
			return compareByTime(entries[i], entries[j])
		}
	case sortSize:
		return func(i, j int) bool {
			return compareBySize(entries[i], entries[j])
		}
	case sortVersion:
		return func(i, j int) bool {
			return versionCompare(entries[i].name, entries[j].name)
		}
	default:
		return func(i, j int) bool {
			return compareByName(entries[i].name, entries[j].name)
		}
	}
}

// reverseEntries reverses the slice in place.
func reverseEntries(entries []lsEntry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}

// compareByName implements C locale name comparison.
// R1.3: LC_COLLATE=C sort order (byte-level comparison).
func compareByName(a, b string) bool {
	return a < b
}

// compareByTime implements newest-first time sort with name tiebreaker.
// R2.5: modification time sort.
func compareByTime(a, b lsEntry) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a.name, b.name)
	}
	if a.info.ModTime.Equal(b.info.ModTime) {
		return compareByName(a.name, b.name)
	}
	return a.info.ModTime.After(b.info.ModTime)
}

// compareBySize implements largest-first size sort with name tiebreaker.
// R2.6: file size sort.
func compareBySize(a, b lsEntry) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a.name, b.name)
	}
	if a.info.Size == b.info.Size {
		return compareByName(a.name, b.name)
	}
	return a.info.Size > b.info.Size
}

// versionCompare implements strverscmp-style natural version sort.
// R2.9: numeric runs compared as numbers.
func versionCompare(a, b string) bool {
	return strverscmp(a, b) < 0
}

// strverscmp compares two strings with natural version ordering.
// Returns negative if a < b, 0 if equal, positive if a > b.
// R2.9: matches glibc strverscmp behavior.
func strverscmp(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigit(ca) && isDigit(cb) {
			r := compareDigitSeq(a[ai:], b[bi:])
			if r != 0 {
				return r
			}
			ai += digitRunLen(a[ai:])
			bi += digitRunLen(b[bi:])
			continue
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		ai++
		bi++
	}
	return len(a) - len(b)
}

// isDigit returns true if b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// compareDigitSeq compares two digit sequences using strverscmp rules.
// Leading zeros trigger fractional comparison; otherwise integral.
func compareDigitSeq(a, b string) int {
	if a[0] == '0' || b[0] == '0' {
		return compareFractional(a, b)
	}
	return compareIntegral(a, b)
}

// compareFractional compares digit sequences with leading zeros.
// Shorter sequence wins when prefix matches (fractional semantics).
func compareFractional(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && isDigit(a[i]) && isDigit(b[i]) {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
		i++
	}
	aMore := i < len(a) && isDigit(a[i])
	bMore := i < len(b) && isDigit(b[i])
	if aMore {
		return 1
	}
	if bMore {
		return -1
	}
	return 0
}

// compareIntegral compares digit sequences as integers.
// Longer run wins; equal-length runs compared digit by digit.
func compareIntegral(a, b string) int {
	aLen := digitRunLen(a)
	bLen := digitRunLen(b)
	if aLen != bLen {
		return aLen - bLen
	}
	for i := 0; i < aLen; i++ {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

// digitRunLen returns the length of the leading digit run in s.
func digitRunLen(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// entryPrefixWidths holds max widths for inode and block count prefixes
// used by non-long output formats.
// R2.11, R2.12: right-aligned prefix columns.
type entryPrefixWidths struct {
	inode  int
	blocks int
}

// computePrefixWidths computes max widths for inode and block count
// prefixes across all entries.
// R2.11, R2.12: column width for prefix alignment.
// R3.7: block width accounts for human-readable format.
func computePrefixWidths(cfg *lsConfig, entries []lsEntry) entryPrefixWidths {
	var pw entryPrefixWidths
	if !cfg.showInode && !cfg.showBlocks {
		return pw
	}
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		if cfg.showInode {
			updateMaxInt(&pw.inode, uintWidth(e.info.Ino))
		}
		if cfg.showBlocks {
			s := formatBlockCount(e.info.Blocks, cfg.humanSize)
			updateMaxInt(&pw.blocks, len(s))
		}
	}
	return pw
}

// formatEntryPrefix builds the inode and/or blocks prefix string for
// non-long output formats.
// R2.11: inode prefix. R2.12: block count prefix. R2.15: order.
// R3.7: human-readable blocks when -h is active.
func formatEntryPrefix(cfg *lsConfig, e lsEntry, pw entryPrefixWidths) string {
	if !cfg.showInode && !cfg.showBlocks {
		return ""
	}
	var sb strings.Builder
	if cfg.showInode && e.info != nil {
		fmt.Fprintf(&sb, "%*d ", pw.inode, e.info.Ino)
	}
	if cfg.showBlocks && e.info != nil {
		bStr := formatBlockCount(e.info.Blocks, cfg.humanSize)
		fmt.Fprintf(&sb, "%*s ", pw.blocks, bStr)
	}
	return sb.String()
}

// formatOutput dispatches to the appropriate output formatter.
// R1.1, R1.5, R1.6, R1.11, R1.13: format selection.
// R2.11, R2.12: computes prefix widths for -i/-s.
func formatOutput(cfg *lsConfig, entries []lsEntry) {
	if len(entries) == 0 {
		return
	}
	pw := computePrefixWidths(cfg, entries)
	switch cfg.format {
	case formatSingle:
		formatSingleColumn(cfg, entries, pw)
	case formatLong:
		formatLongListing(cfg, entries)
	case formatColumns:
		formatMultiColumn(cfg, entries, pw)
	case formatAcross:
		formatAcrossColumns(cfg, entries, pw)
	}
}

// formatSingleColumn prints one entry per line.
// R1.2, R1.5: single-column output.
// R2.11, R2.12: optional inode/blocks prefix.
func formatSingleColumn(cfg *lsConfig, entries []lsEntry, pw entryPrefixWidths) {
	for _, e := range entries {
		fmt.Printf("%s%s\n",
			formatEntryPrefix(cfg, e, pw),
			entryDisplayName(cfg, e))
	}
}

// formatMultiColumn prints entries in vertical multi-column layout.
// R1.1, R1.11, R1.12: vertical column fill.
// R2.11, R2.12: optional inode/blocks prefix in names.
func formatMultiColumn(cfg *lsConfig, entries []lsEntry, pw entryPrefixWidths) {
	names := entryNames(cfg, entries, pw)
	rows := format.Columns(names, cfg.termWidth)
	if len(rows) == 0 {
		return
	}
	colWidths := computeGridWidths(rows)
	printColumnRows(rows, colWidths)
}

// computeGridWidths returns the maximum display width per column.
func computeGridWidths(rows [][]string) []int {
	numCols := len(rows[0])
	widths := make([]int, numCols)
	for _, row := range rows {
		for c, cell := range row {
			w := utf8.RuneCountInString(cell)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	return widths
}

// printColumnRows prints rows with column-aligned entries separated by gaps.
func printColumnRows(rows [][]string, colWidths []int) {
	for _, row := range rows {
		var sb strings.Builder
		lastCol := len(row) - 1
		for c, cell := range row {
			sb.WriteString(cell)
			if c < lastCol {
				pad := colWidths[c] - utf8.RuneCountInString(cell) + 2
				for i := 0; i < pad; i++ {
					sb.WriteByte(' ')
				}
			}
		}
		fmt.Println(sb.String())
	}
}

// formatAcrossColumns prints entries in horizontal multi-column layout.
// R1.13: entries fill across rows first, then down to the next row.
// R1.14: -x is mutually exclusive with -l, -1, -C; last flag wins.
// R2.11, R2.12: optional inode/blocks prefix in names.
func formatAcrossColumns(cfg *lsConfig, entries []lsEntry, pw entryPrefixWidths) {
	names := entryNames(cfg, entries, pw)
	numCols := fitAcrossColumns(names, cfg.termWidth)
	rows := chunkNames(names, numCols)
	if len(rows) == 0 {
		return
	}
	colWidths := computeGridWidths(rows)
	printColumnRows(rows, colWidths)
}

// fitAcrossColumns returns the maximum number of columns that fit within
// termWidth when entries are distributed row-major (across).
// R1.13: column count for -x horizontal layout.
func fitAcrossColumns(names []string, termWidth int) int {
	for numCols := len(names); numCols > 1; numCols-- {
		if acrossGridFits(names, numCols, termWidth) {
			return numCols
		}
	}
	return 1
}

// acrossGridFits checks whether names in numCols columns (row-major)
// fit within the given terminal width.
func acrossGridFits(names []string, numCols, termWidth int) bool {
	colWidths := make([]int, numCols)
	for i, name := range names {
		col := i % numCols
		w := utf8.RuneCountInString(name)
		if w > colWidths[col] {
			colWidths[col] = w
		}
	}
	total := 0
	for i, w := range colWidths {
		total += w
		if i < numCols-1 {
			total += 2
		}
	}
	return total <= termWidth
}

// chunkNames splits names into rows of at most numCols entries each.
// R1.13: row-major distribution for horizontal fill.
func chunkNames(names []string, numCols int) [][]string {
	if len(names) == 0 {
		return nil
	}
	rows := make([][]string, 0, (len(names)+numCols-1)/numCols)
	for i := 0; i < len(names); i += numCols {
		end := i + numCols
		if end > len(names) {
			end = len(names)
		}
		rows = append(rows, names[i:end])
	}
	return rows
}

// formatLongListing prints long-format output with aligned columns.
// R1.6, R1.7, R1.8: long format fields.
func formatLongListing(cfg *lsConfig, entries []lsEntry) {
	cw := computeColumnWidths(cfg, entries)
	for _, e := range entries {
		printLongEntry(cfg, e, cw)
	}
}

// printLongEntry prints a single entry in long format.
// R1.6: permissions nlink owner group size mtime name.
// R2.11: inode prefix when -i. R2.12: blocks prefix when -s.
// R2.14: numeric UID/GID when -n.
func printLongEntry(cfg *lsConfig, e lsEntry, cw columnWidths) {
	if e.info == nil {
		fmt.Println(e.name)
		return
	}
	fi := e.info
	prefix := longEntryPrefix(cfg, fi, cw)
	perm := permissionString(fi.Mode)
	owner := resolveOwner(fi.Uid, cfg.numericIDs)
	group := resolveGroup(fi.Gid, cfg.numericIDs)
	mtime := formatTime(fi.ModTime)
	name := entryDisplayName(cfg, e)
	// R1.10: append symlink target when entry is a symlink.
	// In long format, GNU ls omits the -F "@" indicator for symlinks
	// because the " -> target" already identifies the type.
	if e.link != "" {
		name = coloredName(e)
		name += " -> " + e.link
	}
	// R3.5: use human-readable size when -h is active with -l.
	sizeStr := formatSize(fi.Size, cfg.humanSize)
	fmt.Printf("%s%s %*d %-*s %-*s %*s %s %s\n",
		prefix,
		perm,
		cw.nlink, fi.Nlink,
		cw.owner, owner,
		cw.group, group,
		cw.size, sizeStr,
		mtime,
		name,
	)
}

// longEntryPrefix builds the inode and/or blocks prefix for long format.
// R2.11: inode right-aligned. R2.12: blocks right-aligned.
// R3.7: human-readable blocks when -h is active.
func longEntryPrefix(cfg *lsConfig, fi *sys.FileInfo, cw columnWidths) string {
	if !cfg.showInode && !cfg.showBlocks {
		return ""
	}
	var sb strings.Builder
	if cfg.showInode {
		fmt.Fprintf(&sb, "%*d ", cw.inode, fi.Ino)
	}
	if cfg.showBlocks {
		bStr := formatBlockCount(fi.Blocks, cfg.humanSize)
		fmt.Fprintf(&sb, "%*s ", cw.blocks, bStr)
	}
	return sb.String()
}

// printTotalLine prints the "total N" block count line for long format.
// R1.10: total blocks in 1K-block units (st_blocks/2).
// R2.13: total line also shown when -s is active.
// R3.6: human-readable total when -h is active.
func printTotalLine(cfg *lsConfig, entries []lsEntry) {
	var totalBlocks int64
	for _, e := range entries {
		if e.info != nil {
			totalBlocks += e.info.Blocks
		}
	}
	kb := totalBlocks / 2
	if cfg.humanSize {
		fmt.Printf("total %s\n", gnuHumanSize(kb*1024))
	} else {
		fmt.Printf("total %d\n", kb)
	}
}

// permissionString builds the 10-character permission string.
// R1.6: file type + rwx with setuid/setgid/sticky.
func permissionString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	perm := mode.Perm()
	// R1.6: owner rwx with setuid
	buf[1] = permChar(perm&0o400 != 0, 'r')
	buf[2] = permChar(perm&0o200 != 0, 'w')
	buf[3] = execSpecialChar(perm&0o100 != 0, mode&os.ModeSetuid != 0, 's', 'S')
	// R1.6: group rwx with setgid
	buf[4] = permChar(perm&0o040 != 0, 'r')
	buf[5] = permChar(perm&0o020 != 0, 'w')
	buf[6] = execSpecialChar(perm&0o010 != 0, mode&os.ModeSetgid != 0, 's', 'S')
	// R1.6: other rwx with sticky
	buf[7] = permChar(perm&0o004 != 0, 'r')
	buf[8] = permChar(perm&0o002 != 0, 'w')
	buf[9] = execSpecialChar(perm&0o001 != 0, mode&os.ModeSticky != 0, 't', 'T')
	return string(buf[:])
}

// fileTypeChar returns the leading character for the permission string.
// R1.6: d, l, c, b, p, s, or -.
func fileTypeChar(mode os.FileMode) byte {
	switch {
	case mode&os.ModeDir != 0:
		return 'd'
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return 'c'
	case mode&os.ModeDevice != 0:
		return 'b'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	default:
		return '-'
	}
}

// permChar returns ch if the permission bit is set, '-' otherwise.
func permChar(set bool, ch byte) byte {
	if set {
		return ch
	}
	return '-'
}

// execSpecialChar returns the execute position character accounting for
// setuid/setgid/sticky special bits.
// R1.6: 's'/'S' for setuid/setgid, 't'/'T' for sticky.
func execSpecialChar(exec, special bool, lower, upper byte) byte {
	if special {
		if exec {
			return lower
		}
		return upper
	}
	if exec {
		return 'x'
	}
	return '-'
}

// resolveOwner looks up the username for a UID.
// R1.8: os/user.LookupId with numeric fallback.
// R2.14: returns numeric string when numeric is true.
func resolveOwner(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup looks up the group name for a GID.
// R1.8: os/user.LookupGroupId with numeric fallback.
// R2.14: returns numeric string when numeric is true.
func resolveGroup(gid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// formatTime formats the modification time for long format.
// R1.9: recent vs old time display.
func formatTime(t time.Time) string {
	if time.Since(t) < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// formatSize formats a file size, optionally human-readable.
// R3.5: dispatches to gnuHumanSize when cfg.humanSize is true.
func formatSize(size int64, human bool) string {
	if human {
		return gnuHumanSize(size)
	}
	return strconv.FormatInt(size, 10)
}

// formatBlockCount formats an entry's block count in 1K-block units.
// R2.12: fi.Blocks/2 converts 512-byte to 1024-byte blocks.
// R3.7: optional human-readable format.
func formatBlockCount(blocks int64, human bool) string {
	kb := blocks / 2
	if human {
		return gnuHumanSize(kb * 1024)
	}
	return strconv.FormatInt(kb, 10)
}

// gnuHumanSuffixes are 1024-base unit suffixes matching GNU ls -h.
// GNU ls uses K/M/G (not Ki/Mi/Gi) with 1024-base and ceiling rounding.
var gnuHumanSuffixes = []string{"K", "M", "G", "T", "P", "E"}

// gnuHumanSize formats bytes matching GNU ls -h conventions.
// R3.5, R3.6, R3.7: 1024-base, ceiling rounding, K/M/G suffixes.
func gnuHumanSize(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10)
	}
	val := float64(bytes)
	for i, suffix := range gnuHumanSuffixes {
		val /= 1024.0
		if i == len(gnuHumanSuffixes)-1 || val < 1024.0 {
			return formatGNUValue(val, suffix)
		}
	}
	return strconv.FormatInt(bytes, 10)
}

// formatGNUValue renders a scaled value with GNU-style precision.
// Values >= 10 are integer (ceiling); values < 10 use one decimal.
func formatGNUValue(val float64, suffix string) string {
	if val >= 10 {
		return fmt.Sprintf("%d%s", int64(math.Ceil(val)), suffix)
	}
	rounded := math.Ceil(val*10) / 10
	return fmt.Sprintf("%.1f%s", rounded, suffix)
}

// classifyIndicator returns the -F indicator character for a file mode.
// R3.8: "/" dir, "*" executable, "@" symlink, "|" FIFO, "=" socket.
// R3.9: executable = any execute bit set (owner, group, or other).
func classifyIndicator(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return "/"
	case mode&os.ModeSymlink != 0:
		return "@"
	case mode&os.ModeNamedPipe != 0:
		return "|"
	case mode&os.ModeSocket != 0:
		return "="
	case mode.IsRegular() && mode&0o111 != 0:
		return "*"
	default:
		return ""
	}
}

// coloredName returns the entry name with optional ANSI color wrapping
// but without the -F classify indicator.
func coloredName(e lsEntry) string {
	name := e.name
	if e.info != nil {
		c := format.FileTypeColor(e.info.Mode)
		if c != "" {
			name = c + name + format.Reset()
		}
	}
	return name
}

// entryDisplayName builds the display name with optional color and indicator.
// R3.3, R3.8, R3.10: color wrapping and classification.
func entryDisplayName(cfg *lsConfig, e lsEntry) string {
	name := e.name
	if e.info != nil {
		c := format.FileTypeColor(e.info.Mode)
		if c != "" {
			name = c + name + format.Reset()
		}
	}
	if cfg.classify && e.info != nil {
		name += classifyIndicator(e.info.Mode)
	}
	return name
}

// recurseSubdirs recursively lists subdirectories encountered in entries.
// R3.11: each subdir printed with "PATH:" header and blank-line separators.
// R3.13: symlinks to directories are not followed.
// R3.14: current filter flags apply to each subdirectory.
// R3.15: subdirectories are visited in the current sort order.
func recurseSubdirs(cfg *lsConfig, entries []lsEntry) int {
	exitCode := exitOK
	for _, e := range entries {
		if !e.isDir {
			continue
		}
		// R3.13: skip symlinks to directories
		if e.info != nil && e.info.Mode&os.ModeSymlink != 0 {
			continue
		}
		// Skip . and .. to avoid infinite recursion
		if e.name == "." || e.name == ".." {
			continue
		}
		fmt.Println()
		code := listDir(cfg, e.path, true)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// columnWidths holds the computed widths for aligned long-format columns.
type columnWidths struct {
	nlink  int
	owner  int
	group  int
	size   int
	inode  int
	blocks int
}

// computeColumnWidths calculates alignment widths for long format fields.
// R1.6, R1.7: nlink, owner, group, size widths.
// R2.11: inode column width. R2.12: blocks column width.
// R3.5: size width accounts for human-readable format.
// R3.7: blocks width accounts for human-readable format.
func computeColumnWidths(cfg *lsConfig, entries []lsEntry) columnWidths {
	var cw columnWidths
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		updateMaxInt(&cw.nlink, uintWidth(e.info.Nlink))
		updateMaxInt(&cw.owner, len(resolveOwner(e.info.Uid, cfg.numericIDs)))
		updateMaxInt(&cw.group, len(resolveGroup(e.info.Gid, cfg.numericIDs)))
		updateMaxInt(&cw.size, len(formatSize(e.info.Size, cfg.humanSize)))
		if cfg.showInode {
			updateMaxInt(&cw.inode, uintWidth(e.info.Ino))
		}
		if cfg.showBlocks {
			s := formatBlockCount(e.info.Blocks, cfg.humanSize)
			updateMaxInt(&cw.blocks, len(s))
		}
	}
	return cw
}

// updateMaxInt sets *cur to v if v is larger.
func updateMaxInt(cur *int, v int) {
	if v > *cur {
		*cur = v
	}
}

// uintWidth returns the decimal digit count of a uint64.
func uintWidth(n uint64) int {
	return len(strconv.FormatUint(n, 10))
}

// intWidth returns the decimal digit count of an int64.
func intWidth(n int64) int {
	return len(strconv.FormatInt(n, 10))
}

// entryNames extracts display names from entries with optional prefix.
// R2.11, R2.12: inode/blocks prefix included when flags are active.
func entryNames(cfg *lsConfig, entries []lsEntry, pw entryPrefixWidths) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = formatEntryPrefix(cfg, e, pw) + entryDisplayName(cfg, e)
	}
	return names
}

// osErrMsg extracts the OS-level error message from a path error.
func osErrMsg(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
