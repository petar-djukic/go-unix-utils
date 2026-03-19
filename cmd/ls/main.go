// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1–R1.14, R2.1–R2.15, R3.1–R3.15, R4.1–R4.7:
// directory listing with format modes (-1, -l, -C, -x), C locale sorting,
// dot-file filtering (-a, -A), permission strings, owner/group resolution,
// file metadata via pkg/sys, modification time formatting, total block count,
// symlink display, multi-column output (vertical and horizontal),
// last-format-flag-wins, long format link count, device major/minor,
// timestamps, total block header, inode display (-i), block count display (-s),
// numeric UID/GID (-n), combined -i -s prefix ordering, --color flag support,
// human-readable size display (-h), -F classify indicator, -R recursive listing,
// sort modes (-t, -S, -r, -U, -v), recursive format/filter/sort propagation,
// exit codes (0 success, 1/2 failure), SIGPIPE signal handling,
// SIGWINCH terminal resize handling, -n implies -l, and format flag precedence.
package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// cachedTermWidth holds the terminal width, updated on SIGWINCH.
// R4.5: SIGWINCH handler re-queries terminal width for column layout.
var cachedTermWidth atomic.Int32

const progName = "ls"

// defaultTermWidth is the column width when stdout is not a TTY.
// R1.11: -C uses 80 columns when stdout is not a TTY.
const defaultTermWidth = 80

// tabSize is the tab-stop interval for column alignment.
// GNU ls uses 8-character tab stops for multi-column output indentation.
const tabSize = 8

// formatMode controls the output format.
type formatMode int

const (
	formatColumns    formatMode = iota // multi-column vertical fill (default when TTY, or -C)
	formatSingle                       // one entry per line (-1 or non-TTY default)
	formatLong                         // long format (-l)
	formatHorizontal                   // multi-column horizontal fill (-x)
)

// colorMode controls the --color flag behavior.
// R3.1: supports always, auto, never.
type colorMode int

const (
	colorNever  colorMode = iota // default: no color output
	colorAuto                    // R3.2: colorize only when stdout is a TTY
	colorAlways                  // always colorize output
)

// sortMode controls the entry sort order.
// R2.5-R2.10: sort mode selection for -t, -S, -v, -U flags.
type sortMode int

const (
	sortName    sortMode = iota // default: alphabetical by name
	sortTime                   // R2.5: -t sort by modification time
	sortSize                   // R2.6: -S sort by file size
	sortVersion                // R2.9: -v natural version sort
	sortNone                   // R2.8: -U no sorting, directory order
)

// entry holds a directory entry's name and optional metadata.
type entry struct {
	name string
	path string
	info *sys.FileInfo
}

// options holds parsed flag values for prd008-ls.
type options struct {
	format        formatMode // output format
	showAll       bool       // R2.1: -a include dotfiles and . / ..
	almostAll     bool       // R2.2: -A include dotfiles except . / ..
	showInode     bool       // R2.11: -i prepend inode number
	showBlocks    bool       // R2.12: -s prepend block count
	numericIDs    bool       // R2.14: -n numeric UID/GID (implies -l)
	color         colorMode  // R3.1: color output mode
	humanReadable bool       // R3.5: -h human-readable sizes
	classify      bool       // R3.8: -F append type indicator
	recursive     bool       // R3.11: -R list subdirectories recursively
	sortBy        sortMode   // R2.5-R2.10: sort mode
	reverse       bool       // R2.7: -r reverse sort order
}

// needsStat returns true when entries require metadata beyond just the name.
// R3.3: color needs the file mode to determine color.
// R3.8: -F needs mode for type indicators.
// R3.11: -R needs mode to identify subdirectories.
func needsStat(opts options) bool {
	return opts.format == formatLong || opts.showInode ||
		opts.showBlocks || opts.color != colorNever ||
		opts.classify || opts.recursive ||
		opts.sortBy == sortTime || opts.sortBy == sortSize
}

// longWidths holds column widths for long format alignment.
type longWidths struct {
	nlink  int
	owner  int
	group  int
	size   int  // max width of size for non-device files
	major  int  // max width of major device number
	minor  int  // max width of minor device number
	hasDev bool // true if any device files in listing
	inode  int  // R2.11: max width of inode number
	blocks int  // R2.12: max width of block count
}

// sizeFieldWidth returns the column width for the size/device field.
func (w longWidths) sizeFieldWidth() int {
	if !w.hasDev {
		return w.size
	}
	devWidth := w.major + 2 + w.minor
	if devWidth > w.size {
		return devWidth
	}
	return w.size
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses flags and lists directory entries, returning the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	opts, paths, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	setupColor(opts)
	defer format.ResetColorEnabled()
	initTermWidth()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	return listPaths(paths, opts, stdout, stderr)
}

// initTermWidth caches the terminal width and registers a SIGWINCH handler.
// R4.5: on SIGWINCH, re-queries width via sys.OnTerminalResize callback.
func initTermWidth() {
	cachedTermWidth.Store(int32(getTermWidthDirect()))
	sys.OnTerminalResize(func(width int) {
		cachedTermWidth.Store(int32(width))
	})
}

// getTermWidthDirect queries the terminal width without using the cache.
func getTermWidthDirect() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return defaultTermWidth
	}
	return w
}

// setupColor sets the process-global color mode based on the --color flag.
// R3.1: --color=always enables, --color=never disables.
// R3.2: --color=auto enables only when stdout is a TTY.
// R3.3: calls format.SetColorEnabled to control FileTypeColor/Reset output.
// R3.4: --color=never suppresses all ANSI escape sequences.
func setupColor(opts options) {
	switch opts.color {
	case colorAlways:
		format.SetColorEnabled(true)
	case colorAuto:
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	default:
		format.SetColorEnabled(false)
	}
}

// listPaths lists file and directory arguments in GNU ls order:
// file arguments first (sorted), then each directory's contents.
func listPaths(paths []string, opts options, stdout, stderr io.Writer) int {
	files, dirs, exitCode := classifyArgs(paths, stderr)
	sort.Strings(files)
	sort.Strings(dirs)
	needBlank := false
	if len(files) > 0 {
		ec := printFileArgs(files, opts, stdout)
		if ec > exitCode {
			exitCode = ec
		}
		needBlank = true
	}
	// R4.2: show headers when multiple original args (including failed ones),
	// or when recursive. Matches GNU ls n_files > 1 logic.
	showHeader := len(paths) > 1 || opts.recursive
	for _, d := range dirs {
		if needBlank {
			fmt.Fprintln(stdout)
		}
		ec := listOneDir(d, opts, stdout, stderr, showHeader)
		if ec > exitCode {
			exitCode = ec
		}
		needBlank = true
	}
	return exitCode
}

// printFileArgs prints file arguments (non-directories).
// File argument listings do not include a "total" block line.
func printFileArgs(files []string, opts options, w io.Writer) int {
	entries := make([]entry, 0, len(files))
	for _, f := range files {
		ent := entry{name: f, path: f}
		if needsStat(opts) {
			fi, err := sys.Lstat(f)
			if err != nil {
				continue
			}
			ent.info = fi
		}
		entries = append(entries, ent)
	}
	outputEntries(entries, opts, w, false)
	return 0
}

// classifyArgs separates paths into files and directories by stat.
// Prints errors and sets exit code for inaccessible paths.
func classifyArgs(paths []string, stderr io.Writer) ([]string, []string, int) {
	var files, dirs []string
	exitCode := 0
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(stderr, "%s: cannot access '%s': %s\n",
				progName, p, unwrapErr(err))
			exitCode = 2
			continue
		}
		if fi.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}
	return files, dirs, exitCode
}

// listOneDir lists a single directory's entries with an optional header.
// R3.11: when recursive, subdirectories are listed after the parent.
func listOneDir(dir string, opts options, stdout, stderr io.Writer, showHeader bool) int {
	if showHeader {
		fmt.Fprintf(stdout, "%s:\n", dir)
	}
	entries, err := readEntries(dir, opts)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot open directory '%s': %s\n",
			progName, dir, unwrapErr(err))
		return 2
	}
	outputEntries(entries, opts, stdout, true)
	if opts.recursive {
		return recurseSubdirs(entries, opts, stdout, stderr)
	}
	return 0
}

// recurseSubdirs lists each subdirectory found in entries.
// R3.11: each subdirectory is preceded by a blank line and a header.
func recurseSubdirs(entries []entry, opts options, stdout, stderr io.Writer) int {
	exitCode := 0
	for _, e := range entries {
		if !isRecursibleDir(e) {
			continue
		}
		fmt.Fprintln(stdout)
		ec := listOneDir(e.path, opts, stdout, stderr, true)
		if ec > exitCode {
			exitCode = ec
		}
	}
	return exitCode
}

// isRecursibleDir returns true if the entry is a real subdirectory to recurse.
// Excludes "." and ".." to prevent infinite recursion.
// Does not follow symlinks to directories (R3.13 implicit).
func isRecursibleDir(e entry) bool {
	if e.name == "." || e.name == ".." {
		return false
	}
	if e.info == nil {
		return false
	}
	if e.info.Mode&os.ModeSymlink != 0 {
		return false
	}
	return e.info.Mode&os.ModeDir != 0
}

// readEntries reads directory entries, applies dot-filtering (R1.4, R2.1, R2.2),
// stats entries when needed, and sorts by name in C locale order (R1.3).
func readEntries(dir string, opts options) ([]entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var entries []entry
	// R2.1: -a includes . and .. entries.
	if opts.showAll {
		entries = append(entries, makeDotEntries(dir, opts)...)
	}
	for _, e := range dirEntries {
		if !shouldShow(e.Name(), opts) {
			continue
		}
		fullPath := joinPath(dir, e.Name())
		ent := entry{name: e.Name(), path: fullPath}
		if needsStat(opts) {
			fi, err := sys.Lstat(fullPath)
			if err == nil {
				ent.info = fi
			}
		}
		entries = append(entries, ent)
	}
	sortEntries(entries, opts)
	return entries, nil
}

// makeDotEntries creates . and .. entries with metadata if needed.
// R2.1: -a includes . and .. in the listing.
func makeDotEntries(dir string, opts options) []entry {
	dots := make([]entry, 2)
	for i, name := range []string{".", ".."} {
		fullPath := joinPath(dir, name)
		dots[i] = entry{name: name, path: fullPath}
		if needsStat(opts) {
			fi, err := sys.Lstat(fullPath)
			if err == nil {
				dots[i].info = fi
			}
		}
	}
	return dots
}

// shouldShow returns true if the entry should appear in the listing.
// R1.4: dotfiles hidden by default. R2.1: -a shows all. R2.2: -A shows dotfiles except . and ..
func shouldShow(name string, opts options) bool {
	if len(name) == 0 || name[0] != '.' {
		return true
	}
	return opts.showAll || opts.almostAll
}

// sortEntries sorts entries according to the active sort mode and reverse flag.
// R2.5-R2.10: sort mode selection. R2.8: -U skips sorting.
// R3.15: recursive listing uses the same sort order.
func sortEntries(entries []entry, opts options) {
	if opts.sortBy == sortNone {
		return
	}
	less := nameLess
	switch opts.sortBy {
	case sortTime:
		less = timeLess
	case sortSize:
		less = sizeLess
	case sortVersion:
		less = versionLess
	}
	if opts.reverse {
		orig := less
		less = func(a, b entry) bool { return orig(b, a) }
	}
	sort.Slice(entries, func(i, j int) bool {
		return less(entries[i], entries[j])
	})
}

// nameLess compares entries by name in C locale order.
func nameLess(a, b entry) bool {
	return a.name < b.name
}

// timeLess compares entries by modification time, newest first.
// R2.5: entries with the same mtime use name as tiebreaker.
func timeLess(a, b entry) bool {
	ta, tb := entryModTime(a), entryModTime(b)
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	return a.name < b.name
}

// entryModTime returns the modification time for an entry.
func entryModTime(e entry) time.Time {
	if e.info != nil {
		return e.info.ModTime
	}
	return time.Time{}
}

// sizeLess compares entries by file size, largest first.
// R2.6: entries with the same size use name as tiebreaker.
func sizeLess(a, b entry) bool {
	sa, sb := entrySize(a), entrySize(b)
	if sa != sb {
		return sa > sb
	}
	return a.name < b.name
}

// entrySize returns the file size for an entry.
func entrySize(e entry) int64 {
	if e.info != nil {
		return e.info.Size
	}
	return 0
}

// versionLess compares entries using natural version sort.
// R2.9: digit runs are compared numerically.
func versionLess(a, b entry) bool {
	return strverscmp(a.name, b.name) < 0
}

// strverscmp implements GNU strverscmp natural version comparison.
// Digit runs are compared numerically; other characters byte-by-byte.
func strverscmp(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigitByte(ca) && isDigitByte(cb) {
			cmp := cmpDigitRuns(a, b, &ai, &bi)
			if cmp != 0 {
				return cmp
			}
			continue
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		ai++
		bi++
	}
	return (len(a) - ai) - (len(b) - bi)
}

// cmpDigitRuns compares two digit runs starting at *ai and *bi.
// Advances both indices past the digit runs.
func cmpDigitRuns(a, b string, ai, bi *int) int {
	if a[*ai] == '0' || b[*bi] == '0' {
		return cmpFractionalRun(a, b, ai, bi)
	}
	return cmpIntegralRun(a, b, ai, bi)
}

// cmpFractionalRun compares digit runs with leading zeros digit-by-digit.
func cmpFractionalRun(a, b string, ai, bi *int) int {
	for *ai < len(a) && *bi < len(b) &&
		isDigitByte(a[*ai]) && isDigitByte(b[*bi]) {
		if a[*ai] != b[*bi] {
			r := int(a[*ai]) - int(b[*bi])
			advanceDigits(a, ai)
			advanceDigits(b, bi)
			return r
		}
		(*ai)++
		(*bi)++
	}
	aHas := *ai < len(a) && isDigitByte(a[*ai])
	bHas := *bi < len(b) && isDigitByte(b[*bi])
	advanceDigits(a, ai)
	advanceDigits(b, bi)
	if aHas && !bHas {
		return 1
	}
	if bHas && !aHas {
		return -1
	}
	return 0
}

// cmpIntegralRun compares digit runs without leading zeros as integers.
func cmpIntegralRun(a, b string, ai, bi *int) int {
	aLen := countDigits(a, *ai)
	bLen := countDigits(b, *bi)
	firstDiff := 0
	for *ai < len(a) && *bi < len(b) &&
		isDigitByte(a[*ai]) && isDigitByte(b[*bi]) {
		if firstDiff == 0 && a[*ai] != b[*bi] {
			firstDiff = int(a[*ai]) - int(b[*bi])
		}
		(*ai)++
		(*bi)++
	}
	advanceDigits(a, ai)
	advanceDigits(b, bi)
	if aLen != bLen {
		return aLen - bLen
	}
	return firstDiff
}

// countDigits returns the number of consecutive digit bytes from start.
func countDigits(s string, start int) int {
	n := 0
	for i := start; i < len(s) && isDigitByte(s[i]); i++ {
		n++
	}
	return n
}

// advanceDigits moves *idx past any remaining digit bytes.
func advanceDigits(s string, idx *int) {
	for *idx < len(s) && isDigitByte(s[*idx]) {
		(*idx)++
	}
}

// isDigitByte returns true if b is an ASCII digit.
func isDigitByte(b byte) bool {
	return b >= '0' && b <= '9'
}

// outputEntries dispatches to the appropriate output format.
// R1.14: the format mode is determined by the last format flag on the command line.
func outputEntries(entries []entry, opts options, w io.Writer, showTotal bool) {
	if showTotal && (opts.format == formatLong || opts.showBlocks) {
		printTotalBlocks(entries, opts, w)
	}
	switch opts.format {
	case formatLong:
		printLongLines(entries, opts, w)
	case formatColumns:
		printColumnar(entries, opts, w)
	case formatHorizontal:
		printHorizontal(entries, opts, w)
	default:
		printSingle(entries, opts, w)
	}
}

// printSingle outputs entries one per line.
// R3.3: entry names are colorized when color is active.
// R3.8: -F appends type indicators.
func printSingle(entries []entry, opts options, w io.Writer) {
	names := colorizedDisplayNames(entries, opts)
	for _, n := range names {
		fmt.Fprintln(w, n)
	}
}

// displayNames returns entry display strings for non-long formats (plain, no color).
// Used for column width calculation in multi-column modes.
// R3.8: includes type indicator when -F is active.
func displayNames(entries []entry, opts options) []string {
	if !opts.showInode && !opts.showBlocks {
		return plainNames(entries, opts.classify)
	}
	inodeW, blockW := maxPrefixWidths(entries, opts)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = nonLongPrefix(e, opts, inodeW, blockW) +
			plainEntryName(e, opts.classify)
	}
	return names
}

// colorizedDisplayNames returns display strings with colorized entry names.
// R3.3: wraps entry names with ANSI color codes when color is active.
// R3.8/R3.10: includes type indicator after ANSI reset when -F is active.
func colorizedDisplayNames(entries []entry, opts options) []string {
	if !opts.showInode && !opts.showBlocks {
		return colorizedNames(entries, opts.classify)
	}
	inodeW, blockW := maxPrefixWidths(entries, opts)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = nonLongPrefix(e, opts, inodeW, blockW) +
			colorEntryName(e.name, e.info, opts.classify)
	}
	return names
}

// plainNames returns plain entry names with optional type indicators.
// R3.8: when classify is true, appends type indicator character.
func plainNames(entries []entry, classify bool) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = plainEntryName(e, classify)
	}
	return names
}

// plainEntryName returns the plain display name for an entry.
// R3.8: appends type indicator when classify is true.
func plainEntryName(e entry, classify bool) string {
	if !classify || e.info == nil {
		return e.name
	}
	return e.name + typeIndicator(e.info.Mode)
}

// colorizedNames returns colorized entry names with optional indicators.
// R3.3: wraps names with ANSI codes based on file type.
// R3.10: indicator appears after the ANSI reset code.
func colorizedNames(entries []entry, classify bool) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = colorEntryName(e.name, e.info, classify)
	}
	return names
}

// colorEntryName wraps a name with ANSI color codes based on file type.
// R3.3: colorCode + name + resetCode. Returns plain name when color is off.
// R3.8/R3.10: when classify is true, indicator is appended after reset code.
func colorEntryName(name string, info *sys.FileInfo, classify bool) string {
	indicator := ""
	if classify && info != nil {
		indicator = typeIndicator(info.Mode)
	}
	if info == nil {
		return name + indicator
	}
	c := format.FileTypeColor(info.Mode)
	if c == "" {
		return name + indicator
	}
	return c + name + format.Reset() + indicator
}

// typeIndicator returns the -F classify indicator for a file mode.
// R3.8: "/" directory, "*" executable, "@" symlink, "|" pipe, "=" socket.
// R3.9: executable is any execute bit set (mode & 0o111 != 0).
func typeIndicator(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return "/"
	case mode&os.ModeSymlink != 0:
		return "@"
	case mode&os.ModeNamedPipe != 0:
		return "|"
	case mode&os.ModeSocket != 0:
		return "="
	case mode.Perm()&0o111 != 0:
		return "*"
	default:
		return ""
	}
}

// nonLongPrefix formats the inode/blocks prefix for non-long format entries.
// R2.11: inode right-aligned. R2.12: blocks right-aligned in 1K units.
// R2.15: when both are given, inode is printed first, then block count.
// R3.7: -h converts block counts to human-readable form.
func nonLongPrefix(e entry, opts options, inodeW, blockW int) string {
	prefix := ""
	if opts.showInode {
		ino := uint64(0)
		if e.info != nil {
			ino = e.info.Ino
		}
		prefix += fmt.Sprintf("%*d ", inodeW, ino)
	}
	if opts.showBlocks {
		prefix += formatBlockPrefix(e.info, opts.humanReadable, blockW)
	}
	return prefix
}

// formatBlockPrefix formats a single block count prefix with right alignment.
// R3.7: when humanReadable, uses HumanSize; otherwise plain integer.
func formatBlockPrefix(info *sys.FileInfo, humanReadable bool, width int) string {
	if humanReadable {
		s := "0"
		if info != nil {
			s = humanBlock(info.Blocks)
		}
		return fmt.Sprintf("%*s ", width, s)
	}
	blk := int64(0)
	if info != nil {
		blk = info.Blocks / 2
	}
	return fmt.Sprintf("%*d ", width, blk)
}

// humanBlock converts 512-byte block count to human-readable form.
// R3.6/R3.7: consistent binary algorithm matching GNU coreutils ceiling rounding.
func humanBlock(blocks int64) string {
	return gnuHumanSize(blocks * 512)
}

// gnuHumanSize formats a byte count as a human-readable string matching
// GNU coreutils conventions: ceiling-based rounding, single-decimal for
// values < 10, integer for values >= 10. Uses 1024-based (binary) units.
func gnuHumanSize(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10)
	}
	suffixes := [...]string{"K", "M", "G", "T", "P", "E"}
	value := float64(bytes) / 1024
	for i := 0; i < len(suffixes)-1; i++ {
		if value < 1024 {
			return formatGNUValue(value, suffixes[i])
		}
		value /= 1024
	}
	return formatGNUValue(value, suffixes[len(suffixes)-1])
}

// formatGNUValue formats a scaled value with its suffix using GNU conventions.
// Values < 10 show one decimal (ceiling); values >= 10 show as integer (ceiling).
func formatGNUValue(value float64, suffix string) string {
	if value < 10 {
		v := math.Ceil(value*10) / 10
		return fmt.Sprintf("%.1f%s", v, suffix)
	}
	return fmt.Sprintf("%d%s", int64(math.Ceil(value)), suffix)
}

// maxPrefixWidths computes the maximum inode and block column widths.
func maxPrefixWidths(entries []entry, opts options) (int, int) {
	inodeW, blockW := 0, 0
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		if opts.showInode {
			updateWidth(&inodeW, len(strconv.FormatUint(e.info.Ino, 10)))
		}
		if opts.showBlocks {
			if opts.humanReadable {
				updateWidth(&blockW, len(humanBlock(e.info.Blocks)))
			} else {
				updateWidth(&blockW, len(strconv.FormatInt(e.info.Blocks/2, 10)))
			}
		}
	}
	return inodeW, blockW
}

// printColumnar outputs entries in multi-column layout with vertical fill.
// R3.3: uses plain names for width calculation, colored names for output.
func printColumnar(entries []entry, opts options, w io.Writer) {
	if len(entries) == 0 {
		return
	}
	plainNms := displayNames(entries, opts)
	tw := getTermWidth()
	rows := format.Columns(plainNms, tw)
	colWidths := columnMaxWidths(rows)
	starts := colStartPositions(colWidths)
	coloredNames := colorizedDisplayNames(entries, opts)
	numRows := len(rows)
	coloredRows := buildVerticalRows(coloredNames, numRows)
	for i, row := range coloredRows {
		printRowColored(row, rows[i], starts, w)
	}
}

// buildVerticalRows distributes names into rows using vertical fill order.
// Entry i goes to row i%numRows, column i/numRows.
func buildVerticalRows(names []string, numRows int) [][]string {
	if numRows == 0 {
		return nil
	}
	rows := make([][]string, numRows)
	for i, name := range names {
		r := i % numRows
		rows[r] = append(rows[r], name)
	}
	return rows
}

// printHorizontal outputs entries in multi-column layout with horizontal fill.
// R1.13: entries fill across columns first, then down to the next row.
// R3.3: uses plain names for width calculation, colored names for output.
func printHorizontal(entries []entry, opts options, w io.Writer) {
	if len(entries) == 0 {
		return
	}
	plainNms := displayNames(entries, opts)
	tw := getTermWidth()
	numCols := computeHorizCols(plainNms, tw)
	coloredNames := colorizedDisplayNames(entries, opts)
	if numCols <= 1 {
		for _, n := range coloredNames {
			fmt.Fprintln(w, n)
		}
		return
	}
	widths := horizColWidths(plainNms, numCols)
	starts := colStartPositions(widths)
	for i := 0; i < len(coloredNames); i += numCols {
		cEnd := i + numCols
		if cEnd > len(coloredNames) {
			cEnd = len(coloredNames)
		}
		pEnd := i + numCols
		if pEnd > len(plainNms) {
			pEnd = len(plainNms)
		}
		printRowColored(coloredNames[i:cEnd], plainNms[i:pEnd], starts, w)
	}
}

// computeHorizCols finds the maximum column count for horizontal fill layout.
// R1.13: entries fill left-to-right; column c holds entries at indices c, c+numCols, etc.
func computeHorizCols(names []string, termWidth int) int {
	n := len(names)
	for cols := n; cols > 1; cols-- {
		widths := horizColWidths(names, cols)
		total := 0
		for i, w := range widths {
			total += w
			if i < cols-1 {
				total += 2
			}
		}
		if total <= termWidth {
			return cols
		}
	}
	return 1
}

// horizColWidths computes per-column max widths for horizontal fill.
// Entry i is assigned to column i%numCols.
func horizColWidths(names []string, numCols int) []int {
	widths := make([]int, numCols)
	for i, name := range names {
		c := i % numCols
		w := utf8.RuneCountInString(name)
		if w > widths[c] {
			widths[c] = w
		}
	}
	return widths
}

// columnMaxWidths computes the max rune width per column from row data.
func columnMaxWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	numCols := 0
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	widths := make([]int, numCols)
	for _, row := range rows {
		for c, s := range row {
			w := utf8.RuneCountInString(s)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	return widths
}

// colStartPositions computes the starting cursor position for each column.
// Each column starts at the previous column's start + width + 2-char gap.
func colStartPositions(widths []int) []int {
	starts := make([]int, len(widths))
	pos := 0
	for i, w := range widths {
		starts[i] = pos
		pos += w + 2
	}
	return starts
}

// printRowColored prints a row where coloredRow may contain ANSI codes and
// plainRow contains the corresponding plain text for cursor position tracking.
func printRowColored(coloredRow, plainRow []string, starts []int, w io.Writer) {
	pos := 0
	for c, s := range coloredRow {
		fmt.Fprint(w, s)
		if c < len(plainRow) {
			pos += utf8.RuneCountInString(plainRow[c])
		}
		if c < len(coloredRow)-1 && c+1 < len(starts) {
			indent := indentString(pos, starts[c+1])
			fmt.Fprint(w, indent)
			pos = starts[c+1]
		}
	}
	fmt.Fprintln(w)
}

// indentString returns tabs and spaces to advance from cursor position
// 'from' to 'to', using tab stops for efficiency. Matches GNU ls indent().
func indentString(from, to int) string {
	if from >= to {
		return ""
	}
	var buf []byte
	for {
		nextTab := from + tabSize - from%tabSize
		if nextTab > to {
			break
		}
		buf = append(buf, '\t')
		from = nextTab
	}
	for from < to {
		buf = append(buf, ' ')
		from++
	}
	return string(buf)
}

// getTermWidth returns the cached terminal width, defaulting to 80 for non-TTY.
// R1.11: -C with non-TTY uses 80 columns.
// R4.5: uses the cached value updated by SIGWINCH handler.
func getTermWidth() int {
	w := int(cachedTermWidth.Load())
	if w <= 0 {
		return defaultTermWidth
	}
	return w
}

// printTotalBlocks prints the "total N" line before a directory listing.
// R1.10/R2.13: N = sum(fi.Blocks) / 2, converting 512-byte blocks to 1K blocks.
// R3.6: when -h is active, convert total to human-readable form.
func printTotalBlocks(entries []entry, opts options, w io.Writer) {
	var total int64
	for _, e := range entries {
		if e.info != nil {
			total += e.info.Blocks
		}
	}
	if opts.humanReadable {
		fmt.Fprintf(w, "total %s\n", humanBlock(total))
	} else {
		fmt.Fprintf(w, "total %d\n", total/2)
	}
}

// printLongLines outputs entries in long format with aligned columns.
// R1.6: field order is permissions, nlink, owner, group, size, mtime, name.
// R2.11: -i prepends inode. R2.12: -s prepends block count.
func printLongLines(entries []entry, opts options, w io.Writer) {
	widths := computeWidths(entries, opts)
	for _, e := range entries {
		if e.info == nil {
			fmt.Fprintln(w, e.name)
			continue
		}
		printLongEntry(e, widths, opts, w)
	}
}

// printLongEntry formats a single entry in long format.
// R3.3: name field is colorized when color is active.
// R3.5: -h converts sizes to human-readable form.
// R3.8/R3.10: -F appends type indicator.
func printLongEntry(e entry, w longWidths, opts options, out io.Writer) {
	prefix := longPrefix(e, opts, w)
	owner := ownerString(e.info.Uid, opts.numericIDs)
	group := groupString(e.info.Gid, opts.numericIDs)
	name := formatName(e, opts.classify)
	fmt.Fprintf(out, "%s%s %*d %-*s %-*s %s %s %s\n",
		prefix,
		permString(e.info.Mode),
		w.nlink, e.info.Nlink,
		w.owner, owner,
		w.group, group,
		formatSizeField(e.info, w, opts.humanReadable),
		formatMtime(e.info.ModTime),
		name,
	)
}

// longPrefix formats the inode/blocks prefix for long format entries.
// R2.11: inode right-aligned before permissions.
// R2.12: block count right-aligned before permissions (after inode if both).
// R2.15: when both are given, inode is printed first, then block count.
// R3.7: -h converts block counts to human-readable form.
func longPrefix(e entry, opts options, w longWidths) string {
	prefix := ""
	if opts.showInode {
		prefix += fmt.Sprintf("%*d ", w.inode, e.info.Ino)
	}
	if opts.showBlocks {
		if opts.humanReadable {
			s := humanBlock(e.info.Blocks)
			prefix += fmt.Sprintf("%*s ", w.blocks, s)
		} else {
			prefix += fmt.Sprintf("%*d ", w.blocks, e.info.Blocks/2)
		}
	}
	return prefix
}

// ownerString returns the owner display string.
// R2.14: -n uses numeric UID instead of resolved name.
func ownerString(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return resolveUser(uid)
}

// groupString returns the group display string.
// R2.14: -n uses numeric GID instead of resolved name.
func groupString(gid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return resolveGroup(gid)
}

// formatSizeField returns the formatted size or device number field.
// R2.8: device files display "major, minor" instead of size.
// R3.5: -h converts sizes to human-readable form via format.HumanSize.
func formatSizeField(fi *sys.FileInfo, w longWidths, humanReadable bool) string {
	sw := w.sizeFieldWidth()
	if isDevice(fi.Mode) {
		maj := deviceMajor(fi.Rdev)
		min := deviceMinor(fi.Rdev)
		majorW := sw - 2 - w.minor
		return fmt.Sprintf("%*d, %*d", majorW, maj, w.minor, min)
	}
	if humanReadable {
		return fmt.Sprintf("%*s", sw, gnuHumanSize(fi.Size))
	}
	return fmt.Sprintf("%*d", sw, fi.Size)
}

// formatName returns the display name for an entry in long format.
// R1.10: appends " -> target" for symlinks.
// R3.3: entry name is wrapped with ANSI color codes when color is active.
// R3.8/R3.10: -F appends type indicator (after reset code, before arrow).
// Symlinks in long format do not get the @ indicator because the -> arrow
// and the 'l' permission prefix already indicate the file type.
func formatName(e entry, classify bool) string {
	if e.info != nil && e.info.Mode&os.ModeSymlink != 0 {
		name := colorEntryName(e.name, e.info, false)
		target, err := os.Readlink(e.path)
		if err != nil {
			return name
		}
		return name + " -> " + target
	}
	return colorEntryName(e.name, e.info, classify)
}

// computeWidths calculates the column widths for long format alignment.
// R3.5: when -h, size widths are based on HumanSize output.
// R3.7: when -h, block widths are based on humanBlock output.
func computeWidths(entries []entry, opts options) longWidths {
	var w longWidths
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		computeEntryWidths(e, opts, &w)
	}
	return w
}

// computeEntryWidths updates column widths for a single entry.
func computeEntryWidths(e entry, opts options, w *longWidths) {
	if opts.showInode {
		updateWidth(&w.inode, len(strconv.FormatUint(e.info.Ino, 10)))
	}
	if opts.showBlocks {
		if opts.humanReadable {
			updateWidth(&w.blocks, len(humanBlock(e.info.Blocks)))
		} else {
			updateWidth(&w.blocks, len(strconv.FormatInt(e.info.Blocks/2, 10)))
		}
	}
	updateWidth(&w.nlink, len(strconv.FormatUint(e.info.Nlink, 10)))
	updateWidth(&w.owner, len(ownerString(e.info.Uid, opts.numericIDs)))
	updateWidth(&w.group, len(groupString(e.info.Gid, opts.numericIDs)))
	computeSizeWidth(e.info, opts.humanReadable, w)
}

// computeSizeWidth updates the size/device column widths for an entry.
func computeSizeWidth(fi *sys.FileInfo, humanReadable bool, w *longWidths) {
	if isDevice(fi.Mode) {
		w.hasDev = true
		updateWidth(&w.major, len(strconv.FormatUint(deviceMajor(fi.Rdev), 10)))
		updateWidth(&w.minor, len(strconv.FormatUint(deviceMinor(fi.Rdev), 10)))
		return
	}
	if humanReadable {
		updateWidth(&w.size, len(gnuHumanSize(fi.Size)))
	} else {
		updateWidth(&w.size, len(strconv.FormatInt(fi.Size, 10)))
	}
}

// updateWidth sets *current to val if val is larger.
func updateWidth(current *int, val int) {
	if val > *current {
		*current = val
	}
}

// isDevice returns true if the mode indicates a character or block device.
func isDevice(mode os.FileMode) bool {
	return mode&os.ModeDevice != 0
}

// deviceMajor extracts the major device number from a raw device ID.
// Uses Darwin major() macro: (rdev >> 24) & 0xff.
func deviceMajor(rdev uint64) uint64 {
	return (rdev >> 24) & 0xff
}

// deviceMinor extracts the minor device number from a raw device ID.
// Uses Darwin minor() macro: rdev & 0xffffff.
func deviceMinor(rdev uint64) uint64 {
	return rdev & 0xffffff
}

// permString returns the 10-character permission string for a file mode.
// R1.6: permission string algorithm.
func permString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	fillRWX(buf[1:4], mode, 8, os.ModeSetuid, 's', 'S')
	fillRWX(buf[4:7], mode, 5, os.ModeSetgid, 's', 'S')
	fillRWX(buf[7:10], mode, 2, os.ModeSticky, 't', 'T')
	return string(buf[:])
}

// fileTypeChar returns the single character for the file type indicator.
// R1.6: position 0 of the permission string.
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

// fillRWX fills 3 bytes with r/w/x characters for a permission triplet.
// shift is the bit position of the read bit (8=owner, 5=group, 2=other).
func fillRWX(buf []byte, mode os.FileMode, shift uint,
	specialBit os.FileMode, execSpec, noExecSpec byte) {
	perm := uint32(mode.Perm())
	buf[0] = rwChar(perm, shift, 'r')
	buf[1] = rwChar(perm, shift-1, 'w')
	hasExec := perm&(1<<(shift-2)) != 0
	hasSpecial := mode&specialBit != 0
	buf[2] = execChar(hasExec, hasSpecial, execSpec, noExecSpec)
}

// rwChar returns the character for a read or write permission bit.
func rwChar(perm uint32, bit uint, ch byte) byte {
	if perm&(1<<bit) != 0 {
		return ch
	}
	return '-'
}

// execChar returns the execute position character considering special bits.
func execChar(hasExec, hasSpecial bool, execSpec, noExecSpec byte) byte {
	switch {
	case hasSpecial && hasExec:
		return execSpec
	case hasSpecial:
		return noExecSpec
	case hasExec:
		return 'x'
	default:
		return '-'
	}
}

// resolveUser returns the username for a UID, falling back to numeric string.
// R1.8: owner name resolution via os/user.LookupId.
func resolveUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup returns the group name for a GID, falling back to numeric string.
// R1.8: group name resolution via os/user.LookupGroupId.
func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// formatMtime formats the modification time for long format display.
// R1.9: recent files show "Jan _2 15:04", older files show "Jan _2  2006".
func formatMtime(t time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	if time.Since(t) < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// joinPath joins a directory and entry name, preserving the "./" prefix.
// filepath.Join(".", "name") strips the "./" prefix, but recursive ls
// headers must display "./subdir:" to match GNU ls output.
func joinPath(dir, name string) string {
	return dir + "/" + name
}

// unwrapErr extracts the underlying error message from *os.PathError.
func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// parseArgs separates flags from path arguments.
// Returns parsed options, path list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (options, []string, int) {
	opts := defaultOptions()
	var paths []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(&opts, arg, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			continue
		}
		code := applyShortFlags(&opts, arg, stderr)
		if code >= 0 {
			return opts, nil, code
		}
	}
	// R4.6: -n implies -l unconditionally; cannot be overridden by -1 or -C.
	if opts.numericIDs {
		opts.format = formatLong
	}
	return opts, paths, -1
}

// defaultOptions returns options with format set based on TTY detection.
// R1.1: multi-column when stdout is a TTY.
// R1.2: single-column when stdout is not a TTY.
func defaultOptions() options {
	var opts options
	if sys.IsTerminal(os.Stdout.Fd()) {
		opts.format = formatColumns
	} else {
		opts.format = formatSingle
	}
	return opts
}

// applyShortFlags applies all short flags in a combined argument (e.g., -la).
// Returns exit code >= 0 on error, -1 to continue.
func applyShortFlags(o *options, arg string, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		if !applyShortFlag(o, arg[j]) {
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
			printTryHelp(stderr)
			return 2 // R4.3: exit 2 for invalid options
		}
	}
	return -1
}

// applyShortFlag applies a single-character flag.
// R1.14: format flags (-1, -l, -C, -x) all set opts.format; last one wins.
// Returns false for unrecognized flags.
func applyShortFlag(o *options, ch byte) bool {
	switch ch {
	case 'a': // R2.1: include all dotfiles including . and ..
		o.showAll = true
	case 'A': // R2.2: include dotfiles except . and ..
		o.almostAll = true
	case '1': // R1.5: single-column output
		o.format = formatSingle
	case 'l': // R1.6: long format
		o.format = formatLong
	case 'C': // R1.11: force multi-column output (vertical fill)
		o.format = formatColumns
	case 'x': // R1.13: force multi-column output (horizontal fill)
		o.format = formatHorizontal
	case 'i': // R2.11: prepend inode number
		o.showInode = true
	case 's': // R2.12: prepend block count
		o.showBlocks = true
	case 'n': // R2.14: numeric UID/GID, implies -l
		o.numericIDs = true
		o.format = formatLong
	case 'h': // R3.5: human-readable sizes
		o.humanReadable = true
	case 'F': // R3.8: append type indicator
		o.classify = true
	case 'R': // R3.11: list subdirectories recursively
		o.recursive = true
	case 't': // R2.5: sort by modification time
		o.sortBy = sortTime
	case 'S': // R2.6: sort by file size
		o.sortBy = sortSize
	case 'r': // R2.7: reverse sort order
		o.reverse = true
	case 'U': // R2.8: no sorting, directory order
		o.sortBy = sortNone
	case 'v': // R2.9: natural version sort
		o.sortBy = sortVersion
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// R3.1: handles --color=always, --color=auto, --color=never, bare --color.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(o *options, arg string, stdout, stderr io.Writer) int {
	if arg == "--color" || strings.HasPrefix(arg, "--color=") {
		return applyColorFlag(o, arg, stderr)
	}
	switch arg {
	case "--all":
		o.showAll = true
	case "--almost-all":
		o.almostAll = true
	case "--human-readable":
		o.humanReadable = true
	case "--classify":
		o.classify = true
	case "--recursive":
		o.recursive = true
	case "--reverse":
		o.reverse = true
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 2 // R4.3: exit 2 for invalid options
	}
	return -1
}

// applyColorFlag parses the --color flag value.
// R3.1: bare --color defaults to "always". Accepts always, auto, never
// and GNU ls aliases (yes/force, tty/if-tty, none).
func applyColorFlag(o *options, arg string, stderr io.Writer) int {
	value := "always"
	if i := strings.IndexByte(arg, '='); i >= 0 {
		value = arg[i+1:]
	}
	switch value {
	case "always", "yes", "force":
		o.color = colorAlways
	case "auto", "tty", "if-tty":
		o.color = colorAuto
	case "never", "none":
		o.color = colorNever
	default:
		fmt.Fprintf(stderr, "%s: invalid argument '%s' for '--color'\n",
			progName, value)
		printTryHelp(stderr)
		return 2
	}
	return -1
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "List information about the FILEs (the current directory by default).")
	fmt.Fprintln(w, "Sort entries alphabetically if none of -cftuvSUX nor --sort is specified.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -a, --all                  do not ignore entries starting with .")
	fmt.Fprintln(w, "  -A, --almost-all           do not list implied . and ..")
	fmt.Fprintln(w, "  -C                         list entries by columns")
	fmt.Fprintln(w, "      --color[=WHEN]         colorize the output; WHEN can be 'always'")
	fmt.Fprintln(w, "                               (default if omitted), 'auto', or 'never'")
	fmt.Fprintln(w, "  -F, --classify             append indicator (one of */=>@|) to entries")
	fmt.Fprintln(w, "  -h, --human-readable       with -l and/or -s, print human readable sizes")
	fmt.Fprintln(w, "  -i                         print the index number of each file")
	fmt.Fprintln(w, "  -l                         use a long listing format")
	fmt.Fprintln(w, "  -n                         like -l, but list numeric user and group IDs")
	fmt.Fprintln(w, "  -r, --reverse              reverse order while sorting")
	fmt.Fprintln(w, "  -R, --recursive            list subdirectories recursively")
	fmt.Fprintln(w, "  -s                         print the allocated size of each file, in blocks")
	fmt.Fprintln(w, "  -S                         sort by file size, largest first")
	fmt.Fprintln(w, "  -t                         sort by time, newest first")
	fmt.Fprintln(w, "  -U                         do not sort; list entries in directory order")
	fmt.Fprintln(w, "  -v                         natural sort of (version) numbers within text")
	fmt.Fprintln(w, "  -x                         list entries by lines instead of by columns")
	fmt.Fprintln(w, "  -1                         list one file per line")
	fmt.Fprintln(w, "      --help                 display this help and exit")
	fmt.Fprintln(w, "      --version              output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}
