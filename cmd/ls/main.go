// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1–R1.14, R2.1, R2.2: directory listing with format
// modes (-1, -l, -C, -x), C locale sorting, dot-file filtering (-a, -A),
// permission strings, owner/group resolution, file metadata via pkg/sys,
// modification time formatting, total block count, symlink display,
// multi-column output (vertical and horizontal), and last-format-flag-wins.
package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "ls"

// defaultTermWidth is the column width when stdout is not a TTY.
// R1.11: -C uses 80 columns when stdout is not a TTY.
const defaultTermWidth = 80

// tabSize is the tab-stop interval for column alignment.
// GNU ls uses 8-character tab stops for multi-column output indentation.
const tabSize = 8

// formatMode controls the output format.
// R1.1: multi-column when TTY, R1.2: single-column when not TTY,
// R1.5: -1 forces single-column, R1.6: -l forces long format,
// R1.11: -C forces multi-column, R1.13: -x forces horizontal multi-column.
type formatMode int

const (
	formatColumns    formatMode = iota // multi-column vertical fill (default when TTY, or -C)
	formatSingle                       // one entry per line (-1 or non-TTY default)
	formatLong                         // long format (-l)
	formatHorizontal                   // multi-column horizontal fill (-x)
)

// entry holds a directory entry's name and optional metadata.
// R1.7: info is populated via pkg/sys.Lstat when long format is active.
// R1.10: path is the full filesystem path for os.Readlink on symlinks.
type entry struct {
	name string
	path string
	info *sys.FileInfo
}

// options holds parsed flag values for prd008-ls.
type options struct {
	format    formatMode // R1.1, R1.2, R1.5, R1.6, R1.11, R1.13, R1.14: output format
	showAll   bool       // R2.1: -a include dotfiles and . / ..
	almostAll bool       // R2.2: -A include dotfiles except . / ..
}

// longWidths holds column widths for long format alignment.
// R1.6: nlink and size are right-aligned to the widest value.
// R1.8: owner and group are left-aligned to the widest value.
type longWidths struct {
	nlink int
	owner int
	group int
	size  int
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
	if len(paths) == 0 {
		paths = []string{"."}
	}
	return listPaths(paths, opts, stdout, stderr)
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
	showHeader := len(dirs) > 1 || len(files) > 0
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
		if opts.format == formatLong {
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
// Returns a non-zero exit code on error.
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
	return 0
}

// readEntries reads directory entries, applies dot-filtering (R1.4, R2.1, R2.2),
// stats entries when needed (R1.7), and sorts by name in C locale order (R1.3).
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
		fullPath := filepath.Join(dir, e.Name())
		ent := entry{name: e.Name(), path: fullPath}
		if opts.format == formatLong {
			fi, err := sys.Lstat(fullPath)
			if err == nil {
				ent.info = fi
			}
		}
		entries = append(entries, ent)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})
	return entries, nil
}

// makeDotEntries creates . and .. entries with metadata if needed.
// R2.1: -a includes . and .. in the listing.
func makeDotEntries(dir string, opts options) []entry {
	dots := make([]entry, 2)
	for i, name := range []string{".", ".."} {
		fullPath := filepath.Join(dir, name)
		dots[i] = entry{name: name, path: fullPath}
		if opts.format == formatLong {
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

// outputEntries dispatches to the appropriate output format.
// R1.14: the format mode is determined by the last format flag on the command line.
func outputEntries(entries []entry, opts options, w io.Writer, showTotal bool) {
	switch opts.format {
	case formatLong:
		printLong(entries, w, showTotal)
	case formatColumns:
		printColumnar(entries, w)
	case formatHorizontal:
		printHorizontal(entries, w)
	default:
		printSingle(entries, w)
	}
}

// printSingle outputs entries one per line.
func printSingle(entries []entry, w io.Writer) {
	for _, e := range entries {
		fmt.Fprintln(w, e.name)
	}
}

// entryNames extracts display names from entries.
func entryNames(entries []entry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

// printColumnar outputs entries in multi-column layout with vertical fill.
// R1.11: uses pkg/format.Columns for column-first vertical fill.
// R1.12: entries sort vertically (down columns, then across).
func printColumnar(entries []entry, w io.Writer) {
	if len(entries) == 0 {
		return
	}
	names := entryNames(entries)
	tw := getTermWidth()
	rows := format.Columns(names, tw)
	colWidths := columnMaxWidths(rows)
	starts := colStartPositions(colWidths)
	for _, row := range rows {
		printRow(row, starts, w)
	}
}

// printHorizontal outputs entries in multi-column layout with horizontal fill.
// R1.13: entries fill across columns first, then down to the next row.
func printHorizontal(entries []entry, w io.Writer) {
	if len(entries) == 0 {
		return
	}
	names := entryNames(entries)
	tw := getTermWidth()
	numCols := computeHorizCols(names, tw)
	if numCols <= 1 {
		printSingle(entries, w)
		return
	}
	widths := horizColWidths(names, numCols)
	starts := colStartPositions(widths)
	for i := 0; i < len(names); i += numCols {
		end := i + numCols
		if end > len(names) {
			end = len(names)
		}
		printRow(names[i:end], starts, w)
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

// printRow prints a single row of columnar output using tab-aligned indentation.
// Matches GNU ls indent() behavior: uses tabs to reach tab stops, spaces for remainder.
func printRow(row []string, starts []int, w io.Writer) {
	pos := 0
	for c, s := range row {
		fmt.Fprint(w, s)
		pos += utf8.RuneCountInString(s)
		if c < len(row)-1 && c+1 < len(starts) {
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

// getTermWidth returns the terminal width, defaulting to 80 for non-TTY.
// R1.11: -C with non-TTY uses 80 columns.
func getTermWidth() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return defaultTermWidth
	}
	return w
}

// printLong outputs entries in long format with aligned columns.
// R1.6: field order is permissions, nlink, owner, group, size, mtime, name.
// R1.10: prints "total N" block line when showTotal is true.
func printLong(entries []entry, w io.Writer, showTotal bool) {
	if showTotal {
		printTotalBlocks(entries, w)
	}
	widths := computeWidths(entries)
	for _, e := range entries {
		if e.info == nil {
			fmt.Fprintln(w, e.name)
			continue
		}
		printLongEntry(e, widths, w)
	}
}

// printTotalBlocks prints the "total N" line before a directory listing.
// R1.10: N = sum(fi.Blocks) / 2, converting 512-byte blocks to 1K blocks.
func printTotalBlocks(entries []entry, w io.Writer) {
	var total int64
	for _, e := range entries {
		if e.info != nil {
			total += e.info.Blocks
		}
	}
	fmt.Fprintf(w, "total %d\n", total/2)
}

// printLongEntry formats a single entry in long format.
// R1.6: permissions, nlink (right-aligned), owner (left-aligned),
// group (left-aligned), size (right-aligned), mtime, name.
// R1.10: symlinks display " -> target" after the name.
func printLongEntry(e entry, w longWidths, out io.Writer) {
	owner := resolveUser(e.info.Uid)
	group := resolveGroup(e.info.Gid)
	name := formatName(e)
	fmt.Fprintf(out, "%s %*d %-*s %-*s %*d %s %s\n",
		permString(e.info.Mode),
		w.nlink, e.info.Nlink,
		w.owner, owner,
		w.group, group,
		w.size, e.info.Size,
		formatMtime(e.info.ModTime),
		name,
	)
}

// formatName returns the display name for an entry.
// R1.10: appends " -> target" for symlinks.
func formatName(e entry) string {
	if e.info == nil || e.info.Mode&os.ModeSymlink == 0 {
		return e.name
	}
	target, err := os.Readlink(e.path)
	if err != nil {
		return e.name
	}
	return e.name + " -> " + target
}

// computeWidths calculates the column widths for long format alignment.
// R1.6: nlink and size columns sized to the widest value.
func computeWidths(entries []entry) longWidths {
	var w longWidths
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		updateWidth(&w.nlink, len(strconv.FormatUint(e.info.Nlink, 10)))
		updateWidth(&w.owner, len(resolveUser(e.info.Uid)))
		updateWidth(&w.group, len(resolveGroup(e.info.Gid)))
		updateWidth(&w.size, len(strconv.FormatInt(e.info.Size, 10)))
	}
	return w
}

// updateWidth sets *current to val if val is larger.
func updateWidth(current *int, val int) {
	if val > *current {
		*current = val
	}
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
// R1.6: setuid/setgid/sticky modify the execute position character.
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
// R1.6: setuid→s/S, setgid→s/S, sticky→t/T.
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
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(o *options, arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--all":
		o.showAll = true
	case "--almost-all":
		o.almostAll = true
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
	fmt.Fprintln(w, "  -x                         list entries by lines instead of by columns")
	fmt.Fprintln(w, "  -1                         list one file per line")
	fmt.Fprintln(w, "  -l                         use a long listing format")
	fmt.Fprintln(w, "      --help                 display this help and exit")
	fmt.Fprintln(w, "      --version              output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}
