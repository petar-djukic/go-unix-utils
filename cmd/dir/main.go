// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/dir: list directory contents.
// dir is equivalent to ls -C -b: multi-column output with C-style escaping.
// Implements srd107-dir R1.1-R1.5, R2.1-R2.4.
package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages and --help/--version output.
const programName = "dir"

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
type formatMode int

const (
	fmtColumns formatMode = iota // R1.1: dir defaults to multi-column
	fmtOnePer                    // -1: single column
	fmtLong                      // -l: long format
	fmtAcross                    // -x: multi-column, row-across
)

// sortMode controls entry sort order.
type sortMode int

const (
	sortName    sortMode = iota // default: C locale name
	sortTime                    // -t: mtime newest first
	sortSize                    // -S: size largest first
	sortNone                    // -U: directory order
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
	showInode     bool // -i
	showBlocks    bool // -s
	numericIDs    bool // -n
	classify      bool // -F
}

// dirEntry holds a directory entry name and optional metadata.
type dirEntry struct {
	name string
	fi   *sys.FileInfo
}

// longEntry holds pre-formatted fields for a long-format line.
type longEntry struct {
	inode  string
	blocks string
	perm   string
	nlink  string
	owner  string
	group  string
	size   string
	mtime  string
	disp   string
}

// longWidths holds maximum column widths for long format alignment.
type longWidths struct {
	inode  int
	blocks int
	nlink  int
	owner  int
	group  int
	size   int
}

// --- C-style escaping (R1.2) ---

// escapeC escapes non-printable characters and special characters
// using C-style backslash sequences, matching GNU ls -b behavior.
func escapeC(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		appendEscaped(&b, s[i])
	}
	return b.String()
}

// appendEscaped writes the escaped representation of a byte.
func appendEscaped(b *strings.Builder, c byte) {
	switch c {
	case '\\':
		b.WriteString("\\\\")
	case '\n':
		b.WriteString("\\n")
	case '\r':
		b.WriteString("\\r")
	case '\t':
		b.WriteString("\\t")
	case '\a':
		b.WriteString("\\a")
	case '\b':
		b.WriteString("\\b")
	case '\f':
		b.WriteString("\\f")
	case '\v':
		b.WriteString("\\v")
	case ' ':
		b.WriteString("\\ ")
	default:
		if c < 0x20 || c >= 0x7f {
			fmt.Fprintf(b, "\\%03o", c)
		} else {
			b.WriteByte(c)
		}
	}
}

// --- Flag parsing ---

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
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
				os.Exit(2)
			}
			continue
		}
		if arg == "-" || len(arg) < 2 || arg[0] != '-' {
			paths = append(paths, arg)
			continue
		}
		if err := parseFlags(&opts, arg[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			os.Exit(2)
		}
	}
	return opts, paths
}

// parseColorFlag handles --color[=VALUE].
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
func parseFlags(opts *options, flags string) error {
	for _, ch := range flags {
		if err := applyFlag(opts, ch); err != nil {
			return err
		}
	}
	return nil
}

// applyFlag applies a single flag character to options.
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
	case 'b':
		// R1.2: -b is the default for dir; accept but no-op.
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
	case 'F':
		opts.classify = true
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// --- Color setup ---

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
		opts.showBlocks ||
		opts.classify
}

// --- Entry sorting ---

// sortEntries sorts entries according to the active sort mode.
func sortEntries(entries []dirEntry, opts *options) {
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
func selectLess(entries []dirEntry, mode sortMode) func(i, j int) bool {
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

// timeBeforeName compares entries by mtime, newest first.
func timeBeforeName(a, b dirEntry) bool {
	if a.fi != nil && b.fi != nil {
		if !a.fi.ModTime.Equal(b.fi.ModTime) {
			return a.fi.ModTime.After(b.fi.ModTime)
		}
	}
	return a.name < b.name
}

// sizeBeforeName compares entries by size, largest first.
func sizeBeforeName(a, b dirEntry) bool {
	if a.fi != nil && b.fi != nil {
		if a.fi.Size != b.fi.Size {
			return a.fi.Size > b.fi.Size
		}
	}
	return a.name < b.name
}

// --- Version sort ---

// versionLess returns true when a sorts before b using strverscmp.
func versionLess(a, b string) bool {
	return strverscmp(a, b) < 0
}

// strverscmp compares two strings with natural version ordering.
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

// resolveEntries builds dirEntry items with optional metadata.
func resolveEntries(dir string, names []string, info bool) []dirEntry {
	entries := make([]dirEntry, 0, len(names))
	for _, name := range names {
		e := dirEntry{name: name}
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

// --- Display names ---

// colorUsed tracks whether any color code has been emitted.
var colorUsed bool

// displayNames returns display strings for entries with escaping and color.
// R1.2: applies C-style escaping to all names.
func displayNames(entries []dirEntry, classify bool) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		escaped := escapeC(e.name)
		if e.fi != nil {
			names[i] = colorName(escaped, e.fi.Mode)
			if classify {
				names[i] += classifyIndicator(e.fi.Mode)
			}
		} else {
			names[i] = escaped
		}
	}
	return names
}

// colorName wraps a filename in ANSI color codes based on file mode.
func colorName(name string, mode os.FileMode) string {
	c := format.FileTypeColor(mode)
	r := format.Reset()
	if c == "" || c == r {
		return name
	}
	if !colorUsed {
		colorUsed = true
		return r + c + name + r
	}
	return c + name + r
}

// classifyIndicator returns the -F type indicator for a file mode.
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

// --- Prefix display for -i and -s ---

// buildPrefixedNames builds display names with optional inode/block prefixes.
func buildPrefixedNames(entries []dirEntry, opts *options) []string {
	names := displayNames(entries, opts.classify)
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
func entryPrefix(e dirEntry, opts *options, inodeW, blocksW int) string {
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
func formatBlockCount(blocks int64, opts *options) string {
	kblocks := blocks / 2
	if opts.humanReadable {
		return format.HumanSize(kblocks*1024, format.HumanSizeOpts{Binary: true})
	}
	return strconv.FormatInt(kblocks, 10)
}

// computePrefixWidths returns max widths for inode and block columns.
func computePrefixWidths(entries []dirEntry, opts *options) (int, int) {
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

// --- Terminal width ---

// storedTermWidth holds the current terminal width.
var storedTermWidth atomic.Int32

// initTermWidth queries the initial terminal width and installs
// a SIGWINCH handler.
// R1.1: when stdout is not a TTY, uses default width of 80 columns.
func initTermWidth() {
	w := 80
	if sys.IsTerminal(os.Stdout.Fd()) {
		if tw, err := sys.TerminalWidth(); err == nil && tw > 0 {
			w = tw
		}
	}
	storedTermWidth.Store(int32(w))
	sys.OnTerminalResize(func(width int) {
		storedTermWidth.Store(int32(width))
	})
}

// resolveWidth returns the column width for multi-column layout.
func resolveWidth() int {
	w := int(storedTermWidth.Load())
	if w > 0 {
		return w
	}
	return 80
}

// --- Non-long format output ---

// printEntries outputs entries in the appropriate format.
// R1.1: dir defaults to multi-column (fmtColumns).
func printEntries(entries []dirEntry, opts *options) {
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
	}
}

// printOnePerLine writes one entry per line.
func printOnePerLine(names []string) {
	for _, name := range names {
		fmt.Println(name)
	}
}

// tabSize is the default tab stop size matching GNU ls/dir.
const tabSize = 8

// --- Tab-based indentation (matching GNU ls indent()) ---

// indent writes tabs and spaces to advance from column 'from' to 'to'.
func indent(b *strings.Builder, from, to int) {
	for from < to {
		if tabSize > 0 && to/tabSize > (from+1)/tabSize {
			b.WriteByte('\t')
			from += tabSize - from%tabSize
		} else {
			b.WriteByte(' ')
			from++
		}
	}
}

// --- Vertical column layout ---

// printColumnsVertical formats entries in vertical multi-column layout.
func printColumnsVertical(names []string, width int) {
	n := len(names)
	if n == 0 {
		return
	}
	numCols := findMaxCols(names, n, width)
	numRows := (n + numCols - 1) / numCols
	colWidths := computeColWidths(names, numCols, numRows, n, true)
	printVerticalRows(names, numCols, numRows, n, colWidths)
}

// findMaxCols determines the maximum number of columns that fit.
func findMaxCols(names []string, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		w := totalWidth(names, numCols, numRows, n, true)
		if w <= termWidth {
			return numCols
		}
	}
	return 1
}

// totalWidth computes the total display width for a column layout.
// When vertical is true, entries fill columns top-to-bottom.
func totalWidth(names []string, numCols, numRows, n int, vertical bool) int {
	total := 0
	for col := range numCols {
		maxW := colMaxWidth(names, col, numCols, numRows, n, vertical)
		total += maxW
		if col < numCols-1 {
			total += 2
		}
	}
	return total
}

// colMaxWidth returns the max entry width for a column.
func colMaxWidth(names []string, col, numCols, numRows, n int, vertical bool) int {
	maxW := 0
	for row := range numRows {
		idx := entryIndex(col, row, numCols, numRows, vertical)
		if idx >= n {
			break
		}
		if w := len(names[idx]); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// entryIndex returns the index into names for a given col/row.
func entryIndex(col, row, numCols, numRows int, vertical bool) int {
	if vertical {
		return col*numRows + row
	}
	return row*numCols + col
}

// computeColWidths returns the max width for each column.
func computeColWidths(names []string, numCols, numRows, n int, vertical bool) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		widths[col] = colMaxWidth(names, col, numCols, numRows, n, vertical)
	}
	return widths
}

// printVerticalRows prints all rows of a vertical column layout.
func printVerticalRows(names []string, numCols, numRows, n int, colWidths []int) {
	for row := range numRows {
		printColumnRow(names, row, numCols, numRows, n, colWidths, true)
	}
}

// printColumnRow prints a single row using tab-based indentation.
func printColumnRow(names []string, row, numCols, numRows, n int, colWidths []int, vertical bool) {
	var b strings.Builder
	pos := 0
	first := true
	for col := range numCols {
		idx := entryIndex(col, row, numCols, numRows, vertical)
		if idx >= n {
			break
		}
		if !first {
			target := columnStart(colWidths, col)
			indent(&b, pos, target)
			pos = target
		}
		first = false
		b.WriteString(names[idx])
		pos += len(names[idx])
	}
	fmt.Println(b.String())
}

// columnStart returns the character position where column col begins.
func columnStart(colWidths []int, col int) int {
	pos := 0
	for c := 0; c < col; c++ {
		pos += colWidths[c] + 2
	}
	return pos
}

// --- Horizontal column layout ---

// printColumnsHorizontal formats entries across rows, then down.
func printColumnsHorizontal(names []string, width int) {
	n := len(names)
	if n == 0 {
		return
	}
	numCols := findMaxColsHoriz(names, n, width)
	numRows := (n + numCols - 1) / numCols
	colWidths := computeColWidths(names, numCols, numRows, n, false)
	for row := range numRows {
		printColumnRow(names, row, numCols, numRows, n, colWidths, false)
	}
}

// findMaxColsHoriz finds the max columns for horizontal layout.
func findMaxColsHoriz(names []string, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		w := totalWidth(names, numCols, numRows, n, false)
		if w <= termWidth {
			return numCols
		}
	}
	return 1
}

// --- Long format ---

// printLong outputs entries in long format with aligned columns.
func printLong(entries []dirEntry, dir string, opts *options, showTotal bool) {
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
func buildLongEntries(entries []dirEntry, dir string, opts *options) []longEntry {
	result := make([]longEntry, 0, len(entries))
	for _, e := range entries {
		if e.fi == nil {
			continue
		}
		result = append(result, toLongEntry(e, dir, opts))
	}
	return result
}

// toLongEntry converts one dirEntry to a formatted longEntry.
func toLongEntry(e dirEntry, dir string, opts *options) longEntry {
	le := longEntry{
		perm:  permString(e.fi.Mode),
		nlink: strconv.FormatUint(e.fi.Nlink, 10),
		owner: resolveOwnerField(e.fi.Uid, opts.numericIDs),
		group: resolveGroupField(e.fi.Gid, opts.numericIDs),
		size:  formatSize(e.fi.Size, opts),
		mtime: formatTime(e.fi.ModTime),
		disp:  longDisplayName(e, dir, opts.classify),
	}
	if opts.showInode {
		le.inode = strconv.FormatUint(e.fi.Ino, 10)
	}
	if opts.showBlocks {
		le.blocks = formatBlockCount(e.fi.Blocks, opts)
	}
	return le
}

// longDisplayName builds the name field for long format.
func longDisplayName(e dirEntry, dir string, classify bool) string {
	escaped := escapeC(e.name)
	name := colorName(escaped, e.fi.Mode)
	isSymlink := e.fi.Mode&os.ModeSymlink != 0
	if classify && !isSymlink {
		name += classifyIndicator(e.fi.Mode)
	}
	if isSymlink {
		target, err := os.Readlink(filepath.Join(dir, e.name))
		if err == nil {
			name = name + " -> " + escapeC(target)
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

// printLongLine outputs a single long-format line.
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
func printTotalLine(entries []dirEntry, opts *options) {
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

// --- Permission string ---

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

// --- Owner/group resolution ---

// resolveOwnerField returns the owner display string.
func resolveOwnerField(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return resolveOwner(uid)
}

// resolveGroupField returns the group display string.
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

// --- Size and time formatting ---

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
func formatTime(t time.Time) string {
	if time.Since(t) < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// --- Directory listing ---

// listDir reads and prints the contents of a single directory.
func listDir(path string, opts *options) ([]string, error) {
	names, err := readDirEntryNames(path, opts)
	if err != nil {
		return nil, err
	}
	entries := resolveEntries(path, names, needsInfo(opts))
	sortEntries(entries, opts)
	if opts.format == fmtLong {
		printLong(entries, path, opts, true)
	} else {
		if opts.showBlocks {
			printTotalLine(entries, opts)
		}
		printEntries(entries, opts)
	}
	if !opts.recursive {
		return nil, nil
	}
	return collectSubdirs(path, entries), nil
}

// collectSubdirs returns subdirectory paths from entries.
func collectSubdirs(dir string, entries []dirEntry) []string {
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

// listRecursive descends into subdirectories.
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
			*exitCode = 2
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
	entries := make([]dirEntry, 0, len(files))
	for _, f := range files {
		fi, err := sys.Lstat(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, formatError(f, err))
			*exitCode = 1
			continue
		}
		entries = append(entries, dirEntry{name: f, fi: fi})
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

// capitalizeErr returns the error string with the first letter uppercased.
func capitalizeErr(err error) string {
	s := err.Error()
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatError produces GNU dir-compatible error messages.
func formatError(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("%s: cannot access '%s': %s", programName, pe.Path, capitalizeErr(pe.Err))
	}
	return fmt.Sprintf("%s: cannot access '%s': %s", programName, path, capitalizeErr(err))
}

// --- Entry point ---

// run executes the dir command and returns the exit code.
func run(paths []string, opts *options) int {
	exitCode := 0
	var files, dirs []string
	classifyArgs(paths, opts, &files, &dirs, &exitCode)
	showHeader := needsHeader(len(paths), opts)
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
			exitCode = 2
		} else if opts.recursive {
			listRecursive(children, opts, &exitCode)
		}
		needBlank = true
	}
	return exitCode
}

// needsHeader returns true when directory headers should be printed.
func needsHeader(totalArgs int, opts *options) bool {
	return opts.recursive || totalArgs > 1
}

func main() {
	// R2.4: install SIGPIPE handler.
	sys.InstallSIGPIPEHandler()

	// R1.1: query terminal width.
	initTermWidth()

	opts, paths := parseArgs(os.Args[1:])

	// R1.3: configure color output.
	setupColor(&opts)

	// R1.4: default to current directory.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	os.Exit(run(paths, &opts))
}
