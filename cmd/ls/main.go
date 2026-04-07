// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ls: list directory contents.
// Implements srd008-ls R1.1-R1.10, R2.1-R2.4, R3.1-R3.6.
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
	filterDefault filterMode = iota // skip dot-files
	filterAlmostAll                 // -A: show dot-files except . and ..
	filterAll                       // -a: show all including . and ..
)

// colorMode controls ANSI color output.
type colorMode int

const (
	colorAuto   colorMode = iota // default: color when TTY
	colorAlways                  // --color=always
	colorNever                   // --color=never
)

// options holds parsed command-line flags.
type options struct {
	onePerLine    bool       // -1: force single-column output
	longFormat    bool       // -l: long listing format
	dirOnly       bool       // -d: list directories themselves
	humanReadable bool       // -h: human-readable sizes
	filter        filterMode // -a / -A
	color         colorMode  // --color
}

// lsEntry holds a directory entry name and optional metadata.
type lsEntry struct {
	name string
	fi   *sys.FileInfo
}

// longEntry holds pre-formatted fields for a long-format line.
type longEntry struct {
	perm  string
	nlink string
	owner string
	group string
	size  string
	mtime string
	disp  string // display name (colorized, with symlink target)
}

// longWidths holds maximum column widths for long format alignment.
type longWidths struct {
	nlink int
	owner int
	group int
	size  int
}

// parseArgs separates flags from path arguments.
// R4.3: invalid option exits 2.
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
func applyFlag(opts *options, ch rune) error {
	switch ch {
	case '1':
		opts.onePerLine = true
		opts.longFormat = false
	case 'l':
		opts.longFormat = true
		opts.onePerLine = false
	case 'a':
		opts.filter = filterAll
	case 'A':
		opts.filter = filterAlmostAll
	case 'd':
		opts.dirOnly = true
	case 'h':
		opts.humanReadable = true
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// setupColor configures the global color mode based on --color flag.
// R3.2: auto uses TTY detection. R3.3: calls format.SetColorEnabled.
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
	return opts.longFormat || opts.color != colorNever
}

// listDir reads and prints the contents of a single directory.
func listDir(path string, opts *options) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	names := filterEntries(entries, opts.filter)
	// R1.3: sort in C locale byte order.
	sort.Strings(names)
	lsEntries := resolveEntries(path, names, needsInfo(opts))
	if opts.longFormat {
		printLong(lsEntries, path, opts)
		return nil
	}
	printEntries(lsEntries, opts)
	return nil
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

// printEntries outputs entries in the appropriate non-long format.
// R1.2: single-column when stdout is not a TTY.
// R1.5: -1 forces single-column.
func printEntries(entries []lsEntry, opts *options) {
	if len(entries) == 0 {
		return
	}
	names := displayNames(entries)
	if opts.onePerLine || !sys.IsTerminal(os.Stdout.Fd()) {
		printOnePerLine(names)
		return
	}
	printColumns(names)
}

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
// R3.3: uses format.FileTypeColor and format.Reset.
func colorName(name string, mode os.FileMode) string {
	c := format.FileTypeColor(mode)
	if c == "" {
		return name
	}
	return c + name + format.Reset()
}

// printOnePerLine writes one entry per line.
func printOnePerLine(names []string) {
	for _, name := range names {
		fmt.Println(name)
	}
}

// printColumns formats entries in multi-column layout.
// R1.1: uses terminal width for column fitting.
func printColumns(names []string) {
	width, err := sys.TerminalWidth()
	if err != nil || width <= 0 {
		width = 80
	}
	rows := columnLayout(names, width)
	for _, row := range rows {
		printRow(row, names, width)
	}
}

// columnLayout computes multi-column layout for entries.
// Returns row slices of entry names, sorted column-down.
func columnLayout(names []string, termWidth int) [][]string {
	n := len(names)
	if n == 0 {
		return nil
	}
	bestCols := findMaxCols(names, n, termWidth)
	return buildRows(names, bestCols, n)
}

// findMaxCols determines the maximum number of columns that fit.
func findMaxCols(names []string, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		if totalWidth(names, numCols, numRows, n) <= termWidth {
			return numCols
		}
	}
	return 1
}

// totalWidth computes the display width for a given column layout.
// Uses 2-space gap between columns.
func totalWidth(names []string, numCols, numRows, n int) int {
	total := 0
	for col := range numCols {
		maxW := 0
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			w := len(names[idx])
			if w > maxW {
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

// buildRows arranges entries into rows for column-down layout.
func buildRows(names []string, numCols, n int) [][]string {
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

// printRow prints a single row with per-column padding.
func printRow(row []string, allNames []string, termWidth int) {
	n := len(allNames)
	numCols := findMaxCols(allNames, n, termWidth)
	numRows := (n + numCols - 1) / numCols
	colWidths := computeColWidths(allNames, numCols, numRows, n)

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

// computeColWidths returns the max width for each column.
func computeColWidths(names []string, numCols, numRows, n int) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			w := len(names[idx])
			if w > widths[col] {
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

// --- Long format (R1.6-R1.10) ---

// printLong outputs entries in long format with aligned columns.
// R1.10: prints "total N" line before entries.
func printLong(entries []lsEntry, dir string, opts *options) {
	long := buildLongEntries(entries, dir, opts)
	cols := computeLongWidths(long)
	printTotalLine(entries, opts)
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
func toLongEntry(e lsEntry, dir string, opts *options) longEntry {
	return longEntry{
		perm:  permString(e.fi.Mode),
		nlink: strconv.FormatUint(e.fi.Nlink, 10),
		owner: resolveOwner(e.fi.Uid),
		group: resolveGroup(e.fi.Gid),
		size:  formatSize(e.fi.Size, opts),
		mtime: formatTime(e.fi.ModTime),
		disp:  longDisplayName(e, dir),
	}
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
// D2: numeric fields right-aligned, name fields left-aligned.
func computeLongWidths(entries []longEntry) longWidths {
	var w longWidths
	for _, e := range entries {
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
func printLongLine(e longEntry, w longWidths) {
	fmt.Printf("%s %s %s %s %s %s %s\n",
		e.perm,
		format.PadLeft(e.nlink, w.nlink),
		format.PadRight(e.owner, w.owner),
		format.PadRight(e.group, w.group),
		format.PadLeft(e.size, w.size),
		e.mtime,
		e.disp,
	)
}

// printTotalLine prints the "total N" line for long format.
// R1.10: N = sum(fi.Blocks) / 2 (converts 512-byte to 1K units).
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
// R1.6: type char + owner rwx + group rwx + other rwx with setuid/setgid/sticky.
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

// --- Owner/group resolution (R1.8) ---

// resolveOwner maps a UID to a username, falling back to numeric.
// R1.8: uses os/user.LookupId; on failure uses numeric string.
func resolveOwner(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup maps a GID to a group name, falling back to numeric.
// R1.8: uses os/user.LookupGroupId; on failure uses numeric string.
func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// --- Size and time formatting (R1.9, R3.5) ---

// formatSize formats a file size for long format display.
// R3.5: when -h is active, uses pkg/format.HumanSize with binary units.
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

// --- Path handling ---

// listPath handles a single argument: file or directory.
// R2.3: -d lists directories themselves rather than contents.
func listPath(path string, opts *options) error {
	if opts.dirOnly {
		return listSingleEntry(path, opts)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return listSingleEntry(path, opts)
	}
	return listDir(path, opts)
}

// listSingleEntry prints one entry (file or directory with -d).
func listSingleEntry(path string, opts *options) error {
	fi, err := sys.Lstat(path)
	if err != nil {
		return err
	}
	e := lsEntry{name: path, fi: fi}
	if opts.longFormat {
		le := toLongEntry(e, ".", opts)
		w := computeLongWidths([]longEntry{le})
		printLongLine(le, w)
		return nil
	}
	fmt.Println(colorName(e.name, e.fi.Mode))
	return nil
}

// formatError produces GNU ls-compatible error messages.
func formatError(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("ls: cannot access '%s': %s", pe.Path, pe.Err)
	}
	return fmt.Sprintf("ls: cannot access '%s': %s", path, err)
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

	exitCode := 0
	for _, path := range paths {
		if err := listPath(path, &opts); err != nil {
			fmt.Fprintln(os.Stderr, formatError(path, err))
			exitCode = 1
		}
	}
	// R4.1: exit 0 on success. R4.2: exit 1 on minor error.
	os.Exit(exitCode)
}
