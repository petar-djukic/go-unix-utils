// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/dir implements prd107-dir R1.1-R1.4: list directory contents in
// multi-column format with C-style escaping of non-printable characters.
// dir is equivalent to ls -C -b.
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputFormat selects the output layout mode.
type outputFormat int

const (
	formatColumns outputFormat = iota // -C (default for dir)
	formatSingle                      // -1
	formatLong                        // -l
	formatAcross                      // -x
)

// filterMode selects which entries are visible.
type filterMode int

const (
	filterNoDot     filterMode = iota // default: hide dotfiles
	filterAll                         // -a: show all including . and ..
	filterAlmostAll                   // -A: show dotfiles except . and ..
)

// sortMode selects the primary sort key.
type sortMode int

const (
	sortName    sortMode = iota // default: C locale name sort
	sortTime                    // -t: newest first
	sortSize                    // -S: largest first
	sortNone                    // -U: directory order
	sortVersion                 // -v: version sort
)

const (
	exitOK           = 0
	exitMinor        = 1
	exitSerious      = 2
	defaultTermWidth = 80
)

// dirConfig holds all parsed flag state for a single dir invocation.
type dirConfig struct {
	format     outputFormat
	filter     filterMode
	sortBy     sortMode
	reverse    bool // -r
	dirOnly    bool // -d
	classify   bool // -F
	recursive  bool // -R
	showInode  bool // -i
	showBlocks bool // -s
	humanSize  bool // -h
	numericIDs bool // -n
	termWidth  int
	formatSet  bool // whether format was explicitly set by a flag
}

// dirEntry holds metadata for a single directory entry.
type dirEntry struct {
	name  string
	path  string
	info  *sys.FileInfo
	isDir bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses flags, executes the listing, and returns the exit code.
func run(args []string) int {
	cfg, paths, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dir: %s\n", err)
		fmt.Fprintln(os.Stderr, "Try 'dir --help' for more information.")
		return exitSerious
	}
	return listPaths(cfg, paths)
}

// parseFlags parses command-line arguments into a dirConfig and paths.
// R1.4: defaults to "." when no paths are given.
func parseFlags(args []string) (*dirConfig, []string, error) {
	cfg := &dirConfig{}
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
			if err := parseLongFlag(arg[2:]); err != nil {
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
func parseShortFlags(cfg *dirConfig, flags string) error {
	for _, ch := range flags {
		if err := applyShortFlag(cfg, ch); err != nil {
			return err
		}
	}
	return nil
}

// applyShortFlag applies a single short flag character to the config.
func applyShortFlag(cfg *dirConfig, ch rune) error {
	switch ch {
	case '1':
		cfg.format = formatSingle
		cfg.formatSet = true
	case 'l':
		cfg.format = formatLong
		cfg.formatSet = true
	case 'C':
		cfg.format = formatColumns
		cfg.formatSet = true
	case 'x':
		cfg.format = formatAcross
		cfg.formatSet = true
	case 'a':
		cfg.filter = filterAll
	case 'A':
		cfg.filter = filterAlmostAll
	case 'b':
		// no-op: escape is always on for dir
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
		cfg.formatSet = true
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// parseLongFlag handles a single --flag argument (without the -- prefix).
func parseLongFlag(name string) error {
	if name == "color" || strings.HasPrefix(name, "color=") {
		return parseColorFlag(name)
	}
	return fmt.Errorf("unrecognized option '--%s'", name)
}

// parseColorFlag validates --color[=VALUE] without error.
func parseColorFlag(name string) error {
	if name == "color" {
		return nil
	}
	val := name[6:]
	switch val {
	case "always", "yes", "force", "never", "no", "none", "auto":
		return nil
	default:
		return fmt.Errorf("invalid argument '%s' for '--color'", val)
	}
}

// resolveFormat determines terminal width and default format.
// R1.1: dir defaults to multi-column regardless of whether stdout is a TTY.
func resolveFormat(cfg *dirConfig) {
	if sys.IsTerminal(os.Stdout.Fd()) {
		w, err := sys.TerminalWidth()
		if err == nil {
			cfg.termWidth = w
		} else {
			cfg.termWidth = defaultTermWidth
		}
	} else {
		cfg.termWidth = defaultTermWidth
	}
	if !cfg.formatSet {
		cfg.format = formatColumns
	}
}

// listPaths lists each path argument and returns the exit code.
func listPaths(cfg *dirConfig, paths []string) int {
	exitCode := exitOK
	var files []dirEntry
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
	showHeader := len(paths) > 1 || cfg.recursive
	code := listDirs(cfg, dirs, showHeader, len(files) > 0)
	if code > exitCode {
		exitCode = code
	}
	return exitCode
}

// listDirs iterates over directories and lists each one.
func listDirs(cfg *dirConfig, dirs []string, hdr, blank bool) int {
	exitCode := exitOK
	for i, d := range dirs {
		if blank || i > 0 {
			fmt.Println()
		}
		code := listDir(cfg, d, hdr)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// classifyArg stats a path and classifies it as a file or directory.
func classifyArg(
	cfg *dirConfig, path string,
	files *[]dirEntry, dirs *[]string,
) int {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dir: cannot access '%s': %s\n",
			path, osErrMsg(err))
		return exitSerious
	}
	if fi.Mode.IsDir() && !cfg.dirOnly {
		*dirs = append(*dirs, path)
		return exitOK
	}
	*files = append(*files, dirEntry{
		name: path, path: path, info: fi, isDir: fi.Mode.IsDir(),
	})
	return exitOK
}

// listDir reads and lists the contents of a single directory.
func listDir(cfg *dirConfig, dirPath string, showHeader bool) int {
	if showHeader {
		fmt.Printf("%s:\n", dirPath)
	}
	entries, exitCode := readEntries(dirPath)
	if cfg.filter == filterAll {
		entries = addDotEntries(dirPath, entries)
	}
	entries = filterEntries(entries, cfg.filter)
	sortEntries(entries, cfg)
	formatOutput(cfg, entries)
	if cfg.recursive {
		code := recurseSubdirs(cfg, entries)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

// readEntries reads directory entries and stats each one.
func readEntries(dirPath string) ([]dirEntry, int) {
	des, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dir: cannot open directory '%s': %s\n",
			dirPath, osErrMsg(err))
		return nil, exitSerious
	}
	return statDirEntries(des, dirPath)
}

// statDirEntries stats each directory entry and builds dirEntry values.
func statDirEntries(des []os.DirEntry, dir string) ([]dirEntry, int) {
	exitCode := exitOK
	entries := make([]dirEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		path := dir + "/" + name
		fi, err := sys.Lstat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dir: cannot access '%s': %s\n",
				path, osErrMsg(err))
			exitCode = exitMinor
			continue
		}
		entries = append(entries, dirEntry{
			name: name, path: path, info: fi, isDir: fi.Mode.IsDir(),
		})
	}
	return entries, exitCode
}

// addDotEntries prepends "." and ".." entries for -a mode.
func addDotEntries(dirPath string, entries []dirEntry) []dirEntry {
	dots := make([]dirEntry, 0, 2+len(entries))
	for _, name := range []string{".", ".."} {
		path := dirPath + "/" + name
		fi, err := sys.Lstat(path)
		if err != nil {
			continue
		}
		dots = append(dots, dirEntry{
			name: name, path: path, info: fi, isDir: fi.Mode.IsDir(),
		})
	}
	return append(dots, entries...)
}

// filterEntries applies the filter mode to remove hidden entries.
// R1.3: hide dotfiles by default.
func filterEntries(entries []dirEntry, fm filterMode) []dirEntry {
	if fm == filterAll {
		return entries
	}
	result := make([]dirEntry, 0, len(entries))
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
// R1.3: C locale sort by default.
func sortEntries(entries []dirEntry, cfg *dirConfig) {
	if cfg.sortBy == sortNone {
		if cfg.reverse {
			reverseEntries(entries)
		}
		return
	}
	less := sortLessFunc(entries, cfg.sortBy)
	if cfg.reverse {
		orig := less
		less = func(i, j int) bool { return orig(j, i) }
	}
	sort.SliceStable(entries, less)
}

// sortLessFunc returns the comparison function for the given sort mode.
func sortLessFunc(entries []dirEntry, sm sortMode) func(int, int) bool {
	switch sm {
	case sortTime:
		return func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.info != nil && b.info != nil {
				if !a.info.ModTime.Equal(b.info.ModTime) {
					return a.info.ModTime.After(b.info.ModTime)
				}
			}
			return a.name < b.name
		}
	case sortSize:
		return func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.info != nil && b.info != nil {
				if a.info.Size != b.info.Size {
					return a.info.Size > b.info.Size
				}
			}
			return a.name < b.name
		}
	default:
		return func(i, j int) bool {
			return entries[i].name < entries[j].name
		}
	}
}

// reverseEntries reverses the slice in place.
func reverseEntries(entries []dirEntry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}

// formatOutput dispatches to the appropriate output formatter.
func formatOutput(cfg *dirConfig, entries []dirEntry) {
	if len(entries) == 0 {
		return
	}
	switch cfg.format {
	case formatColumns, formatAcross:
		formatMultiColumn(cfg, entries)
	default:
		formatSingleColumn(entries)
	}
}

// formatSingleColumn prints one entry per line with C-style escaping.
func formatSingleColumn(entries []dirEntry) {
	for _, e := range entries {
		fmt.Println(escapeName(e.name))
	}
}

// formatMultiColumn prints entries in vertical multi-column layout.
// R1.1: multi-column format. R1.2: C-style escaping.
func formatMultiColumn(cfg *dirConfig, entries []dirEntry) {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = escapeName(e.name)
	}
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

// printColumnRows prints rows with column-aligned entries using
// tab-based indentation matching GNU coreutils output.
func printColumnRows(rows [][]string, colWidths []int) {
	colStarts := computeColStarts(colWidths)
	for _, row := range rows {
		var sb strings.Builder
		curPos := 0
		for c, cell := range row {
			sb.WriteString(cell)
			curPos += utf8.RuneCountInString(cell)
			if c < len(row)-1 {
				target := colStarts[c+1]
				curPos = indentTo(&sb, curPos, target)
			}
		}
		fmt.Println(sb.String())
	}
}

// computeColStarts returns the display-column start position for each column.
func computeColStarts(colWidths []int) []int {
	starts := make([]int, len(colWidths))
	pos := 0
	for c, w := range colWidths {
		starts[c] = pos
		pos += w + 2 // 2-character gap between columns
	}
	return starts
}

// indentTo advances output from position 'from' to position 'to' using
// a mix of tab characters and spaces, matching GNU coreutils indent().
func indentTo(sb *strings.Builder, from, to int) int {
	const tabSize = 8
	for from < to {
		if to/tabSize > (from+1)/tabSize {
			sb.WriteByte('\t')
			from += tabSize - from%tabSize
		} else {
			sb.WriteByte(' ')
			from++
		}
	}
	return from
}

// escapeName applies C-style backslash escaping to a filename.
// R1.2: escape non-printable characters and backslashes.
func escapeName(name string) string {
	var sb strings.Builder
	sb.Grow(len(name))
	for i := 0; i < len(name); i++ {
		b := name[i]
		if esc, ok := escapeChar(b); ok {
			sb.WriteString(esc)
		} else if b < 0x20 || b >= 0x7f {
			fmt.Fprintf(&sb, "\\%03o", b)
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

// escapeChar returns the C-style escape for named control characters.
func escapeChar(b byte) (string, bool) {
	switch b {
	case '\\':
		return "\\\\", true
	case '\a':
		return "\\a", true
	case '\b':
		return "\\b", true
	case '\f':
		return "\\f", true
	case '\n':
		return "\\n", true
	case '\r':
		return "\\r", true
	case '\t':
		return "\\t", true
	case '\v':
		return "\\v", true
	default:
		return "", false
	}
}

// recurseSubdirs recursively lists subdirectories.
func recurseSubdirs(cfg *dirConfig, entries []dirEntry) int {
	exitCode := exitOK
	for _, e := range entries {
		if !e.isDir || e.name == "." || e.name == ".." {
			continue
		}
		if e.info != nil && e.info.Mode&os.ModeSymlink != 0 {
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

// osErrMsg extracts the OS-level error message from a path error.
func osErrMsg(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeFirst(pe.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst uppercases the first ASCII letter of s.
func capitalizeFirst(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-32) + s[1:]
}
