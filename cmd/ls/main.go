// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ls: list directory contents.
// Implements srd008-ls R1.1-R1.14, R2.1-R2.14, R3.1-R3.6, R3.11-R3.14.
package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// filterMode controls which entries are shown.
type filterMode int

const (
	filterDefault   filterMode = iota // skip dot-files
	filterAlmostAll                   // -A: show dot-files except . and ..
	filterAll                         // -a: show all including . and ..
)

// colorMode controls ANSI color output.
type colorMode int

const (
	colorAuto   colorMode = iota // default: color when TTY
	colorAlways                  // --color=always
	colorNever                   // --color=never
)

// formatMode controls output format.
// R1.14: -C, -x, -l, -1 are mutually exclusive; last flag wins.
type formatMode int

const (
	fmtDefault formatMode = iota // auto-detect based on TTY
	fmtOnePer                    // -1: single column
	fmtLong                      // -l: long format
	fmtColumns                   // -C: multi-column, column-down
	fmtAcross                    // -x: multi-column, row-across
)

// sortMode controls entry sort order.
type sortMode int

const (
	sortName    sortMode = iota // default: C locale name
	sortTime                    // -t: mtime newest first
	sortSize                    // -S: size largest first
	sortNone                    // -U: directory order (no sorting)
	sortVersion                 // -v: natural version sort
)

// options holds parsed command-line flags.
type options struct {
	format        formatMode
	dirOnly       bool
	humanReadable bool
	recursive     bool
	reverse       bool
	filter        filterMode
	color         colorMode
	sortBy        sortMode
	showInode     bool // R2.11: -i
	showBlocks    bool // R2.12: -s
	numericIDs    bool // R2.14: -n
}

// lsEntry holds a directory entry name and optional metadata.
type lsEntry struct {
	name string
	fi   *sys.FileInfo
}

// longEntry holds pre-formatted fields for a long-format line.
type longEntry struct {
	inode  string // R2.11: inode number (empty when -i not given)
	blocks string // R2.12: block count (empty when -s not given)
	perm   string
	nlink  string
	owner  string
	group  string
	size   string
	mtime  string
	disp   string // display name (colorized, with symlink target)
}

// longWidths holds maximum column widths for long format alignment.
type longWidths struct {
	inode  int // R2.11
	blocks int // R2.12
	nlink  int
	owner  int
	group  int
	size   int
}

// --- Flag parsing (R4.3) ---

// parseArgs separates flags from path arguments.
func parseArgs(args []string) (options, []string) {
	var opts options
	var paths []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--color" || strings.HasPrefix(arg, "--color=") {
			if err := parseColorFlag(&opts, arg); err != nil {
				fmt.Fprintf(os.Stderr, "ls: %s\n", err)
				os.Exit(2)
			}
			continue
		}
		if arg == "-" || len(arg) < 2 || arg[0] != '-' {
			paths = append(paths, arg)
			continue
		}
		if err := parseFlags(&opts, arg[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "ls: %s\n", err)
			os.Exit(2)
		}
	}
	return opts, paths
}

// parseColorFlag handles --color[=VALUE].
// R3.1: --color without value defaults to "always".
func parseColorFlag(opts *options, arg string) error {
	if arg == "--color" {
		opts.color = colorAlways
		return nil
	}
	val := arg[len("--color="):]
	switch val {
	case "always", "yes", "force":
		opts.color = colorAlways
	case "never", "no", "none":
		opts.color = colorNever
	case "auto", "tty", "if-tty":
		opts.color = colorAuto
	default:
		return fmt.Errorf("invalid argument '%s' for '--color'", val)
	}
	return nil
}

// parseFlags processes combined short flags.
// R2.4: last of -a/-A wins. R1.14: last format flag wins.
func parseFlags(opts *options, flags string) error {
	for _, ch := range flags {
		if err := applyFlag(opts, ch); err != nil {
			return err
		}
	}
	return nil
}

// applyFlag applies a single flag character to options.
// R1.14: -C, -x, -l, -1 are mutually exclusive; last wins.
// R2.14/R4.6: -n implies -l.
func applyFlag(opts *options, ch rune) error {
	switch ch {
	case '1':
		opts.format = fmtOnePer
	case 'l':
		opts.format = fmtLong
	case 'C':
		opts.format = fmtColumns
	case 'x':
		opts.format = fmtAcross
	case 'a':
		opts.filter = filterAll
	case 'A':
		opts.filter = filterAlmostAll
	case 'd':
		opts.dirOnly = true
	case 'h':
		opts.humanReadable = true
	case 'R':
		opts.recursive = true
	case 'r':
		opts.reverse = true
	case 't':
		opts.sortBy = sortTime
	case 'S':
		opts.sortBy = sortSize
	case 'U':
		opts.sortBy = sortNone
	case 'v':
		opts.sortBy = sortVersion
	case 'i':
		opts.showInode = true
	case 's':
		opts.showBlocks = true
	case 'n':
		opts.numericIDs = true
		opts.format = fmtLong
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// --- Color setup (R3.1-R3.4) ---

// setupColor configures the global color mode based on --color flag.
func setupColor(opts *options) {
	switch opts.color {
	case colorAlways:
		format.SetColorEnabled(true)
	case colorNever:
		format.SetColorEnabled(false)
	case colorAuto:
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	}
}

// needsInfo returns true when file metadata is required for output.
func needsInfo(opts *options) bool {
	return opts.format == fmtLong ||
		opts.color != colorNever ||
		opts.sortBy != sortName ||
		opts.showInode ||
		opts.showBlocks
}

// --- Entry sorting (R2.5, R2.6, R2.7, R2.8, R2.9, R2.10) ---

// sortEntries sorts entries according to the active sort mode.
// R2.8: sortNone skips sorting entirely.
// R2.7: when reverse is true, the comparison is inverted.
func sortEntries(entries []lsEntry, opts *options) {
	if opts.sortBy == sortNone {
		return
	}
	less := selectLess(entries, opts.sortBy)
	if opts.reverse {
		fwd := less
		less = func(i, j int) bool { return fwd(j, i) }
	}
	sort.SliceStable(entries, less)
}

// selectLess returns the comparison function for the given sort mode.
func selectLess(entries []lsEntry, mode sortMode) func(i, j int) bool {
	switch mode {
	case sortTime:
		return func(i, j int) bool {
			return timeBeforeName(entries[i], entries[j])
		}
	case sortSize:
		return func(i, j int) bool {
			return sizeBeforeName(entries[i], entries[j])
		}
	case sortVersion:
		return func(i, j int) bool {
			return versionLess(entries[i].name, entries[j].name)
		}
	default:
		return func(i, j int) bool {
			return entries[i].name < entries[j].name
		}
	}
}

// timeBeforeName returns true when a should appear before b in time sort.
// R2.5: newest first, with C locale name as tiebreaker.
func timeBeforeName(a, b lsEntry) bool {
	if a.fi != nil && b.fi != nil {
		if !a.fi.ModTime.Equal(b.fi.ModTime) {
			return a.fi.ModTime.After(b.fi.ModTime)
		}
	}
	return a.name < b.name
}

// sizeBeforeName returns true when a should appear before b in size sort.
// R2.6: largest first, with C locale name as tiebreaker.
func sizeBeforeName(a, b lsEntry) bool {
	if a.fi != nil && b.fi != nil {
		if a.fi.Size != b.fi.Size {
			return a.fi.Size > b.fi.Size
		}
	}
	return a.name < b.name
}

// --- Version sort (R2.9) ---

// versionLess returns true when a sorts before b using strverscmp semantics.
// Digit runs are compared numerically so "file2" < "file10".
func versionLess(a, b string) bool {
	return strverscmp(a, b) < 0
}

// strverscmp compares two strings with natural version ordering.
// Returns negative if a < b, 0 if equal, positive if a > b.
func strverscmp(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) {
		ca, cb := a[i], b[i]
		if isDigit(ca) && isDigit(cb) {
			if cmp := compareDigitRuns(a[i:], b[i:]); cmp != 0 {
				return cmp
			}
			i += digitRunLen(a[i:])
			continue
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		i++
	}
	return len(a) - len(b)
}

// compareDigitRuns compares two strings that start with digit runs.
// Leading zeros make a run sort before shorter-or-equal numeric value.
func compareDigitRuns(a, b string) int {
	la, lb := digitRunLen(a), digitRunLen(b)
	na, nb := trimLeadingZeros(a[:la]), trimLeadingZeros(b[:lb])
	if len(na) != len(nb) {
		return len(na) - len(nb)
	}
	for k := range len(na) {
		if na[k] != nb[k] {
			return int(na[k]) - int(nb[k])
		}
	}
	// Equal numeric value: more leading zeros sorts first.
	return la - lb
}

// digitRunLen returns the length of the leading digit run in s.
func digitRunLen(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// trimLeadingZeros strips leading '0' characters from a digit string.
func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}

// isDigit returns true when c is an ASCII digit.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// --- Entry filtering and resolution ---

// readDirEntryNames returns filtered entry names from a directory.
// R2.8: when sort mode is sortNone, returns names in filesystem order.
func readDirEntryNames(path string, opts *options) ([]string, error) {
	if opts.sortBy == sortNone {
		return readDirNamesUnsorted(path, opts.filter)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return filterEntries(entries, opts.filter), nil
}

// readDirNamesUnsorted reads directory names in filesystem order.
// R2.8: -U requires directory order, which os.ReadDir does not provide.
func readDirNamesUnsorted(path string, mode filterMode) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, err := f.Readdirnames(-1)
	f.Close() // best-effort close after reading
	if err != nil {
		return nil, err
	}
	return filterNames(raw, mode), nil
}

// filterEntries extracts entry names and applies the filter mode.
// R1.4: default hides dot-files. R2.1: -a includes . and ..
// R2.2: -A includes dot-files except . and ..
func filterEntries(entries []os.DirEntry, mode filterMode) []string {
	var names []string
	if mode == filterAll {
		names = append(names, ".", "..")
	}
	for _, e := range entries {
		name := e.Name()
		if mode == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// filterNames applies the filter mode to raw name strings.
// Used when reading with readDirNames (unsorted) for -U.
func filterNames(rawNames []string, mode filterMode) []string {
	var names []string
	if mode == filterAll {
		names = append(names, ".", "..")
	}
	for _, name := range rawNames {
		if mode == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// resolveEntries builds lsEntry items with optional metadata.
// R1.7: uses pkg/sys.Lstat for each entry.
func resolveEntries(dir string, names []string, info bool) []lsEntry {
	entries := make([]lsEntry, 0, len(names))
	for _, name := range names {
		e := lsEntry{name: name}
		if info {
			fi, err := sys.Lstat(filepath.Join(dir, name))
			if err == nil {
				e.fi = fi
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// --- Display names and colors (R3.3) ---

// displayNames returns display strings for entries, colorized if enabled.
func displayNames(entries []lsEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		if e.fi != nil {
			names[i] = colorName(e.name, e.fi.Mode)
		} else {
			names[i] = e.name
		}
	}
	return names
}

// colorName wraps a filename in ANSI color codes based on file mode.
func colorName(name string, mode os.FileMode) string {
	c := format.FileTypeColor(mode)
	if c == "" {
		return name
	}
	return c + name + format.Reset()
}

// --- Prefix display for -i and -s (R2.11, R2.12, R2.15) ---

// buildPrefixedNames builds display names with optional inode/block prefixes.
// R2.15: when both -i and -s are given, inode is printed first.
func buildPrefixedNames(entries []lsEntry, opts *options) []string {
	names := displayNames(entries)
	if !opts.showInode && !opts.showBlocks {
		return names
	}
	inodeW, blocksW := computePrefixWidths(entries, opts)
	result := make([]string, len(names))
	for i, e := range entries {
		result[i] = entryPrefix(e, opts, inodeW, blocksW) + names[i]
	}
	return result
}

// entryPrefix builds the inode/blocks prefix string for one entry.
func entryPrefix(e lsEntry, opts *options, inodeW, blocksW int) string {
	var prefix string
	if opts.showInode {
		ino := "0"
		if e.fi != nil {
			ino = strconv.FormatUint(e.fi.Ino, 10)
		}
		prefix += fmt.Sprintf("%*s ", inodeW, ino)
	}
	if opts.showBlocks {
		blk := "0"
		if e.fi != nil {
			blk = formatBlockCount(e.fi.Blocks, opts)
		}
		prefix += fmt.Sprintf("%*s ", blocksW, blk)
	}
	return prefix
}

// formatBlockCount converts 512-byte blocks to display string.
// R2.12: report in 1024-byte block units (fi.Blocks / 2).
func formatBlockCount(blocks int64, opts *options) string {
	kblocks := blocks / 2
	if opts.humanReadable {
		return format.HumanSize(kblocks*1024, format.HumanSizeOpts{Binary: true})
	}
	return strconv.FormatInt(kblocks, 10)
}

// computePrefixWidths returns max widths for inode and block columns.
func computePrefixWidths(entries []lsEntry, opts *options) (int, int) {
	inodeW, blocksW := 0, 0
	for _, e := range entries {
		if e.fi == nil {
			continue
		}
		if opts.showInode {
			w := len(strconv.FormatUint(e.fi.Ino, 10))
			inodeW = maxInt(inodeW, w)
		}
		if opts.showBlocks {
			w := len(formatBlockCount(e.fi.Blocks, opts))
			blocksW = maxInt(blocksW, w)
		}
	}
	return inodeW, blocksW
}

// --- Non-long format output ---

// resolveWidth returns the column width for multi-column layout.
// R1.11: when -C or -x forces multi-column on non-TTY, uses 80.
func resolveWidth() int {
	if sys.IsTerminal(os.Stdout.Fd()) {
		w, err := sys.TerminalWidth()
		if err == nil && w > 0 {
			return w
		}
	}
	return 80
}

// printEntries outputs entries in the appropriate non-long format.
// R1.2: single-column when stdout is not a TTY (default mode).
// R1.5: -1 forces single-column.
// R1.11: -C forces vertical multi-column.
// R1.13: -x forces horizontal multi-column.
func printEntries(entries []lsEntry, opts *options) {
	if len(entries) == 0 {
		return
	}
	names := buildPrefixedNames(entries, opts)
	switch opts.format {
	case fmtOnePer:
		printOnePerLine(names)
	case fmtColumns:
		printColumnsVertical(names, resolveWidth())
	case fmtAcross:
		printColumnsHorizontal(names, resolveWidth())
	default:
		if sys.IsTerminal(os.Stdout.Fd()) {
			printColumnsVertical(names, resolveWidth())
		} else {
			printOnePerLine(names)
		}
	}
}

// printOnePerLine writes one entry per line.
func printOnePerLine(names []string) {
	for _, name := range names {
		fmt.Println(name)
	}
}

// --- Vertical column layout (R1.1, R1.11, R1.12) ---

// printColumnsVertical formats entries in vertical multi-column layout.
// Entries are sorted down columns, then across.
func printColumnsVertical(names []string, width int) {
	rows := verticalLayout(names, width)
	for _, row := range rows {
		printRow(row, names, width)
	}
}

// verticalLayout computes multi-column layout for entries.
// Returns row slices of entry names, sorted column-down.
func verticalLayout(names []string, termWidth int) [][]string {
	n := len(names)
	if n == 0 {
		return nil
	}
	bestCols := findMaxCols(names, n, termWidth)
	return buildVerticalRows(names, bestCols, n)
}

// findMaxCols determines the maximum number of columns that fit.
func findMaxCols(names []string, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		if verticalTotalWidth(names, numCols, numRows, n) <= termWidth {
			return numCols
		}
	}
	return 1
}

// verticalTotalWidth computes the display width for a vertical layout.
// Uses 2-space gap between columns.
func verticalTotalWidth(names []string, numCols, numRows, n int) int {
	total := 0
	for col := range numCols {
		maxW := 0
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			if w := len(names[idx]); w > maxW {
				maxW = w
			}
		}
		total += maxW
		if col < numCols-1 {
			total += 2
		}
	}
	return total
}

// buildVerticalRows arranges entries into rows for column-down layout.
func buildVerticalRows(names []string, numCols, n int) [][]string {
	numRows := (n + numCols - 1) / numCols
	rows := make([][]string, numRows)
	for row := range numRows {
		var r []string
		for col := range numCols {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			r = append(r, names[idx])
		}
		rows[row] = r
	}
	return rows
}

// printRow prints a single row with per-column padding (vertical layout).
func printRow(row []string, allNames []string, termWidth int) {
	n := len(allNames)
	numCols := findMaxCols(allNames, n, termWidth)
	numRows := (n + numCols - 1) / numCols
	colWidths := verticalColWidths(allNames, numCols, numRows, n)

	for i, name := range row {
		if i > 0 {
			fmt.Print("  ")
		}
		if i < len(row)-1 {
			fmt.Print(padRight(name, colWidths[i]))
		} else {
			fmt.Print(name)
		}
	}
	fmt.Println()
}

// verticalColWidths returns the max width for each column (vertical).
func verticalColWidths(names []string, numCols, numRows, n int) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			if w := len(names[idx]); w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// padRight pads s with spaces to reach width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// --- Horizontal column layout (R1.13) ---

// printColumnsHorizontal formats entries across rows, then down.
// R1.13: -x sorts entries horizontally (left-to-right, top-to-bottom).
func printColumnsHorizontal(names []string, width int) {
	n := len(names)
	if n == 0 {
		return
	}
	numCols := findMaxColsHoriz(names, n, width)
	numRows := (n + numCols - 1) / numCols
	colWidths := horizColWidths(names, numCols, numRows, n)
	for row := range numRows {
		printHorizRow(names, row, numCols, n, colWidths)
	}
}

// findMaxColsHoriz finds the max columns for horizontal layout.
func findMaxColsHoriz(names []string, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		if horizTotalWidth(names, numCols, numRows, n) <= termWidth {
			return numCols
		}
	}
	return 1
}

// horizTotalWidth computes width for a horizontal column layout.
func horizTotalWidth(names []string, numCols, numRows, n int) int {
	total := 0
	for col := range numCols {
		maxW := 0
		for row := range numRows {
			idx := row*numCols + col
			if idx >= n {
				break
			}
			if w := len(names[idx]); w > maxW {
				maxW = w
			}
		}
		total += maxW
		if col < numCols-1 {
			total += 2
		}
	}
	return total
}

// horizColWidths computes per-column max widths for horizontal layout.
func horizColWidths(names []string, numCols, numRows, n int) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		for row := range numRows {
			idx := row*numCols + col
			if idx >= n {
				break
			}
			if w := len(names[idx]); w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// printHorizRow prints a single row of horizontal layout.
func printHorizRow(names []string, row, numCols, n int, colWidths []int) {
	first := true
	for col := range numCols {
		idx := row*numCols + col
		if idx >= n {
			break
		}
		if !first {
			fmt.Print("  ")
		}
		first = false
		isLast := col == numCols-1 || idx+1 >= n
		if !isLast {
			fmt.Print(padRight(names[idx], colWidths[col]))
		} else {
			fmt.Print(names[idx])
		}
	}
	fmt.Println()
}

// --- Long format (R1.6-R1.10, R2.11-R2.14) ---

// printLong outputs entries in long format with aligned columns.
// R1.10: prints "total N" line before entries when showTotal is true.
func printLong(entries []lsEntry, dir string, opts *options, showTotal bool) {
	long := buildLongEntries(entries, dir, opts)
	cols := computeLongWidths(long)
	if showTotal {
		printTotalLine(entries, opts)
	}
	for _, le := range long {
		printLongLine(le, cols)
	}
}

// buildLongEntries pre-formats fields for each entry.
func buildLongEntries(entries []lsEntry, dir string, opts *options) []longEntry {
	result := make([]longEntry, 0, len(entries))
	for _, e := range entries {
		if e.fi == nil {
			continue
		}
		result = append(result, toLongEntry(e, dir, opts))
	}
	return result
}

// toLongEntry converts one lsEntry to a formatted longEntry.
// R2.11: populates inode when -i is set.
// R2.12: populates blocks when -s is set.
// R2.14: uses numeric UID/GID when -n is set.
func toLongEntry(e lsEntry, dir string, opts *options) longEntry {
	le := longEntry{
		perm:  permString(e.fi.Mode),
		nlink: strconv.FormatUint(e.fi.Nlink, 10),
		owner: resolveOwnerField(e.fi.Uid, opts.numericIDs),
		group: resolveGroupField(e.fi.Gid, opts.numericIDs),
		size:  formatSize(e.fi.Size, opts),
		mtime: formatTime(e.fi.ModTime),
		disp:  longDisplayName(e, dir),
	}
	if opts.showInode {
		le.inode = strconv.FormatUint(e.fi.Ino, 10)
	}
	if opts.showBlocks {
		le.blocks = formatBlockCount(e.fi.Blocks, opts)
	}
	return le
}

// longDisplayName builds the name field for long format including
// color and symlink target. R1.10: appends " -> target" for symlinks.
func longDisplayName(e lsEntry, dir string) string {
	name := colorName(e.name, e.fi.Mode)
	if e.fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(filepath.Join(dir, e.name))
		if err == nil {
			name = name + " -> " + target
		}
	}
	return name
}

// computeLongWidths scans all entries to find maximum column widths.
func computeLongWidths(entries []longEntry) longWidths {
	var w longWidths
	for _, e := range entries {
		w.inode = maxInt(w.inode, len(e.inode))
		w.blocks = maxInt(w.blocks, len(e.blocks))
		w.nlink = maxInt(w.nlink, len(e.nlink))
		w.owner = maxInt(w.owner, len(e.owner))
		w.group = maxInt(w.group, len(e.group))
		w.size = maxInt(w.size, len(e.size))
	}
	return w
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// printLongLine outputs a single long-format line with proper alignment.
// R2.11: prints inode prefix. R2.12: prints blocks prefix.
// R2.15: inode before blocks when both are present.
func printLongLine(e longEntry, w longWidths) {
	prefix := longLinePrefix(e, w)
	fmt.Printf("%s%s %s %s %s %s %s %s\n",
		prefix,
		e.perm,
		format.PadLeft(e.nlink, w.nlink),
		format.PadRight(e.owner, w.owner),
		format.PadRight(e.group, w.group),
		format.PadLeft(e.size, w.size),
		e.mtime,
		e.disp,
	)
}

// longLinePrefix builds the inode/blocks prefix for a long-format line.
func longLinePrefix(e longEntry, w longWidths) string {
	var prefix string
	if w.inode > 0 {
		prefix += format.PadLeft(e.inode, w.inode) + " "
	}
	if w.blocks > 0 {
		prefix += format.PadLeft(e.blocks, w.blocks) + " "
	}
	return prefix
}

// printTotalLine prints the "total N" line for long format and -s.
// R1.10: N = sum(fi.Blocks) / 2 (converts 512-byte to 1K units).
// R2.13: -s with -l uses the same total line.
// R3.6: when -h is active, format total as human-readable.
func printTotalLine(entries []lsEntry, opts *options) {
	var total int64
	for _, e := range entries {
		if e.fi != nil {
			total += e.fi.Blocks
		}
	}
	kblocks := total / 2
	if opts.humanReadable {
		humanStr := format.HumanSize(kblocks*1024, format.HumanSizeOpts{Binary: true})
		fmt.Printf("total %s\n", humanStr)
	} else {
		fmt.Printf("total %d\n", kblocks)
	}
}

// --- Permission string (R1.6) ---

// permString builds a 10-character permission string from a file mode.
func permString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	writeRWX(&buf, 1, mode, 0o400, 0o200, 0o100, os.ModeSetuid, 's', 'S')
	writeRWX(&buf, 4, mode, 0o040, 0o020, 0o010, os.ModeSetgid, 's', 'S')
	writeRWX(&buf, 7, mode, 0o004, 0o002, 0o001, os.ModeSticky, 't', 'T')
	return string(buf[:])
}

// fileTypeChar returns the type indicator character for position 0.
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

// writeRWX fills 3 bytes of the permission string for one class.
func writeRWX(buf *[10]byte, off int, mode os.FileMode,
	rBit, wBit, xBit os.FileMode,
	special os.FileMode, sLower, sUpper byte) {

	buf[off] = permBit(mode, rBit, 'r')
	buf[off+1] = permBit(mode, wBit, 'w')
	x := permBit(mode, xBit, 'x')
	if mode&special != 0 {
		if x == 'x' {
			x = sLower
		} else {
			x = sUpper
		}
	}
	buf[off+2] = x
}

// permBit returns ch if bit is set in mode, '-' otherwise.
func permBit(mode os.FileMode, bit os.FileMode, ch byte) byte {
	if mode&bit != 0 {
		return ch
	}
	return '-'
}

// --- Owner/group resolution (R1.8, R2.14) ---

// resolveOwnerField returns the owner display string.
// R2.14: numeric mode skips name lookup.
func resolveOwnerField(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return resolveOwner(uid)
}

// resolveGroupField returns the group display string.
// R2.14: numeric mode skips name lookup.
func resolveGroupField(gid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return resolveGroup(gid)
}

// resolveOwner maps a UID to a username, falling back to numeric.
func resolveOwner(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup maps a GID to a group name, falling back to numeric.
func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// --- Size and time formatting (R1.9, R3.5) ---

// formatSize formats a file size for long format display.
func formatSize(size int64, opts *options) string {
	if opts.humanReadable {
		return format.HumanSize(size, format.HumanSizeOpts{Binary: true})
	}
	return strconv.FormatInt(size, 10)
}

// sixMonths is approximately six months for time format selection.
const sixMonths = 6 * 30 * 24 * time.Hour

// formatTime formats a modification time for long format display.
// R1.9: recent times show "Jan _2 15:04"; older show "Jan _2  2006".
func formatTime(t time.Time) string {
	if time.Since(t) < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// --- Directory listing ---

// listDir reads and prints the contents of a single directory.
// Returns subdirectory paths when opts.recursive is true.
// R2.8: when sortBy is sortNone, reads entries in directory order.
func listDir(path string, opts *options) ([]string, error) {
	names, err := readDirEntryNames(path, opts)
	if err != nil {
		return nil, err
	}
	lsEntries := resolveEntries(path, names, needsInfo(opts))
	sortEntries(lsEntries, opts)
	if opts.format == fmtLong {
		printLong(lsEntries, path, opts, true)
	} else {
		if opts.showBlocks {
			printTotalLine(lsEntries, opts)
		}
		printEntries(lsEntries, opts)
	}
	if !opts.recursive {
		return nil, nil
	}
	return collectSubdirsFromEntries(path, lsEntries), nil
}

// collectSubdirsFromEntries returns subdirectory paths from sorted entries.
// R3.13: uses Lstat metadata so symlinks to directories are not followed.
// R3.15: preserves the sort order of entries.
func collectSubdirsFromEntries(dir string, entries []lsEntry) []string {
	var dirs []string
	for _, e := range entries {
		if e.name == "." || e.name == ".." {
			continue
		}
		full := filepath.Join(dir, e.name)
		if e.fi != nil {
			if e.fi.Mode.IsDir() {
				dirs = append(dirs, full)
			}
			continue
		}
		info, err := os.Lstat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, full)
	}
	return dirs
}

// listRecursive descends into subdirectories and lists each.
// R3.11: prints path header and blank line before each subdirectory.
func listRecursive(subdirs []string, opts *options, exitCode *int) {
	for _, d := range subdirs {
		fmt.Println()
		fmt.Printf("%s:\n", d)
		children, err := listDir(d, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, formatError(d, err))
			*exitCode = 1
			continue
		}
		listRecursive(children, opts, exitCode)
	}
}

// --- Multi-argument handling ---

// classifyArgs separates paths into files and directories.
func classifyArgs(paths []string, opts *options, files, dirs *[]string, exitCode *int) {
	for _, p := range paths {
		if opts.dirOnly {
			*files = append(*files, p)
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, formatError(p, err))
			*exitCode = 1
			continue
		}
		if info.IsDir() {
			*dirs = append(*dirs, p)
		} else {
			*files = append(*files, p)
		}
	}
}

// listFileArgs lists file arguments (non-directories) as entries.
func listFileArgs(files []string, opts *options, exitCode *int) {
	entries := make([]lsEntry, 0, len(files))
	for _, f := range files {
		fi, err := sys.Lstat(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, formatError(f, err))
			*exitCode = 1
			continue
		}
		entries = append(entries, lsEntry{name: f, fi: fi})
	}
	sortEntries(entries, opts)
	if len(entries) == 0 {
		return
	}
	if opts.format == fmtLong {
		printLong(entries, ".", opts, false)
	} else {
		printEntries(entries, opts)
	}
}

// --- Error formatting ---

// formatError produces GNU ls-compatible error messages.
func formatError(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("ls: cannot access '%s': %s", pe.Path, pe.Err)
	}
	return fmt.Sprintf("ls: cannot access '%s': %s", path, err)
}

// --- Entry point ---

// run executes the ls command and returns the exit code.
func run(paths []string, opts *options) int {
	exitCode := 0
	var files, dirs []string
	classifyArgs(paths, opts, &files, &dirs, &exitCode)
	showHeader := needsHeader(files, dirs, opts)
	needBlank := false
	if len(files) > 0 {
		listFileArgs(files, opts, &exitCode)
		needBlank = true
	}
	for _, d := range dirs {
		if needBlank {
			fmt.Println()
		}
		if showHeader {
			fmt.Printf("%s:\n", d)
		}
		children, err := listDir(d, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, formatError(d, err))
			exitCode = 1
		} else if opts.recursive {
			listRecursive(children, opts, &exitCode)
		}
		needBlank = true
	}
	return exitCode
}

// needsHeader returns true when directory headers should be printed.
func needsHeader(files, dirs []string, opts *options) bool {
	if opts.recursive {
		return true
	}
	if len(dirs) > 1 {
		return true
	}
	return len(files) > 0 && len(dirs) > 0
}

func main() {
	// R4.4: install SIGPIPE handler.
	sys.InstallSIGPIPEHandler()

	opts, paths := parseArgs(os.Args[1:])

	// R3.1-R3.4: configure color output.
	setupColor(&opts)

	// R1.1: default to current directory.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// R4.1: exit 0 on success. R4.2: exit 1 on minor error.
	os.Exit(run(paths, &opts))
}
