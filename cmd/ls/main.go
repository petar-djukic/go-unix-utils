// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1-R1.6: basic directory listing with output modes.
// R1.1: multi-column output when stdout is a TTY.
// R1.2: single-column output when stdout is not a TTY.
// R1.3: C locale sort order (LC_COLLATE=C).
// R1.4: hide dot-entries by default; -a shows all, -A shows almost all.
// R1.5: -1 forces single-column output.
// R1.6: -l long format with permissions, nlink, owner, group, size, mtime.
// R1.11-R1.12: -C forces multi-column, vertical sort.
// -R recursive listing.
package main

import (
	"fmt"
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

// outputMode selects the listing format.
type outputMode int

const (
	modeDefault outputMode = iota
	modeSingle             // -1: one entry per line
	modeLong               // -l: long format
	modeColumns            // -C: forced multi-column
)

// filterMode selects which entries to show.
type filterMode int

const (
	filterDefault filterMode = iota // hide dot-entries
	filterAlmostAll                 // -A: show dot-entries except . and ..
	filterAll                       // -a: show all including . and ..
)

// lsConfig holds parsed command-line options.
type lsConfig struct {
	output    outputMode
	filter    filterMode
	recursive bool // -R
	args      []string
}

// defaultTermWidth is used when stdout is not a TTY and -C is forced.
const defaultTermWidth = 80

func main() {
	sys.InstallSIGPIPEHandler()
	// D5: set LC_ALL=C for consistent collation.
	os.Setenv("LC_ALL", "C")

	cfg := parseArgs(os.Args[1:])
	os.Exit(run(cfg))
}

// parseArgs extracts flags and positional arguments.
func parseArgs(args []string) lsConfig {
	var cfg lsConfig
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			break
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("ls (go-unix-utils) dev")
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
			parseShortFlags(arg[1:], &cfg)
			continue
		}
		if handleLongFlag(arg, &cfg) {
			continue
		}
		cfg.args = append(cfg.args, arg)
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"."}
	}
	return cfg
}

// parseShortFlags processes a cluster of short flags (e.g., "-laR").
func parseShortFlags(flags string, cfg *lsConfig) {
	for _, ch := range flags {
		switch ch {
		case 'a':
			cfg.filter = filterAll
		case 'A':
			cfg.filter = filterAlmostAll
		case 'l':
			cfg.output = modeLong
		case '1':
			cfg.output = modeSingle
		case 'C':
			cfg.output = modeColumns
		case 'R':
			cfg.recursive = true
		}
	}
}

// handleLongFlag processes long-form flags. Returns true if recognized.
func handleLongFlag(arg string, cfg *lsConfig) bool {
	switch arg {
	case "--all":
		cfg.filter = filterAll
	case "--almost-all":
		cfg.filter = filterAlmostAll
	case "--recursive":
		cfg.recursive = true
	default:
		return false
	}
	return true
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).

  -a, --all            do not ignore entries starting with .
  -A, --almost-all     do not list implied . and ..
  -C                   list entries by columns
  -l                   use a long listing format
  -1                   list one file per line
  -R, --recursive      list subdirectories recursively
      --help           display this help and exit
      --version        output version information and exit
`)
}

// run processes all arguments and returns the exit code.
func run(cfg lsConfig) int {
	exitCode := 0
	// Separate files and directories.
	var files []string
	var dirs []string
	for _, arg := range cfg.args {
		fi, err := sys.Stat(arg)
		if err != nil {
			reportError("cannot access", arg, err)
			exitCode = 1
			continue
		}
		if fi.Mode.IsDir() {
			dirs = append(dirs, arg)
		} else {
			files = append(files, arg)
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)

	// Print file arguments first.
	if len(files) > 0 {
		if exitCode |= listFiles(files, cfg); exitCode != 0 {
			exitCode = 1
		}
	}

	multipleTargets := len(files) > 0 || len(dirs) > 1
	needBlank := len(files) > 0

	for _, dir := range dirs {
		if needBlank {
			fmt.Println()
		}
		if multipleTargets || cfg.recursive {
			fmt.Printf("%s:\n", dir)
		}
		if code := listDir(dir, cfg); code != 0 {
			exitCode = 1
		}
		needBlank = true
	}
	return exitCode
}

// listFiles prints non-directory file arguments.
func listFiles(files []string, cfg lsConfig) int {
	if cfg.output == modeLong {
		return printLongEntries(files)
	}
	printEntries(files, cfg)
	return 0
}

// listDir lists the contents of a single directory.
func listDir(dir string, cfg lsConfig) int {
	exitCode := 0
	rawEntries, err := os.ReadDir(dir)
	if err != nil {
		reportError("cannot open directory", dir, err)
		return 1
	}

	names := filterEntries(rawEntries, cfg.filter)
	sort.Strings(names)

	paths := buildPaths(dir, names)

	if cfg.output == modeLong {
		if code := printLongDir(paths, names); code != 0 {
			exitCode = 1
		}
	} else {
		printEntries(names, cfg)
	}

	if cfg.recursive {
		exitCode |= recurseSubdirs(dir, rawEntries, cfg)
	}
	return exitCode
}

// filterEntries returns entry names matching the filter mode.
// R1.4: default hides dot-entries.
// R2.1: -a shows all including . and ..
// R2.2: -A shows dot-entries except . and ..
func filterEntries(entries []os.DirEntry, filter filterMode) []string {
	var names []string
	if filter == filterAll {
		names = append(names, ".", "..")
	}
	for _, e := range entries {
		name := e.Name()
		if filter == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// buildPaths constructs full paths for directory entries.
func buildPaths(dir string, names []string) []string {
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = joinPath(dir, name)
	}
	return paths
}

// joinPath concatenates parent and child, avoiding double slashes.
func joinPath(parent, child string) string {
	if strings.HasSuffix(parent, "/") {
		return parent + child
	}
	return parent + "/" + child
}

// printEntries outputs entries in the selected mode (single or multi-column).
func printEntries(names []string, cfg lsConfig) {
	if len(names) == 0 {
		return
	}
	mode := resolveOutputMode(cfg)
	if mode == modeColumns {
		printColumnar(names)
		return
	}
	// Single-column output.
	for _, name := range names {
		fmt.Println(name)
	}
}

// resolveOutputMode determines the effective output mode.
// R1.1/R1.2: default is multi-column when TTY, single-column otherwise.
// R1.5: -1 forces single-column.
// R1.11: -C forces multi-column.
func resolveOutputMode(cfg lsConfig) outputMode {
	if cfg.output != modeDefault {
		return cfg.output
	}
	if sys.IsTerminal(os.Stdout.Fd()) {
		return modeColumns
	}
	return modeSingle
}

// printColumnar prints entries in multi-column format.
func printColumnar(names []string) {
	width := termWidthOrDefault()
	rows := format.Columns(names, width)
	colWidths := computeColumnWidths(rows)
	for _, row := range rows {
		printColumnarRow(row, colWidths)
	}
}

// termWidthOrDefault returns terminal width, or defaultTermWidth if unavailable.
func termWidthOrDefault() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return defaultTermWidth
	}
	return w
}

// computeColumnWidths returns the max width per column across all rows.
func computeColumnWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	widths := make([]int, maxCols)
	for _, row := range rows {
		for col, entry := range row {
			w := utf8.RuneCountInString(entry)
			if w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// printColumnarRow prints a single row of multi-column output.
func printColumnarRow(row []string, colWidths []int) {
	for i, entry := range row {
		if i < len(row)-1 {
			fmt.Print(format.PadRight(entry, colWidths[i]+2))
		} else {
			fmt.Print(entry)
		}
	}
	fmt.Println()
}

// printLongDir prints directory entries in long format with total line.
// R1.10: prints "total N" line before entries.
func printLongDir(paths, names []string) int {
	infos := make([]*sys.FileInfo, len(paths))
	exitCode := 0
	var totalBlocks int64

	for i, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			reportError("cannot access", names[i], err)
			exitCode = 1
			continue
		}
		infos[i] = fi
		totalBlocks += fi.Blocks
	}

	// R1.10: total line in 1K-block units.
	fmt.Printf("total %d\n", totalBlocks/2)

	printLongLines(names, paths, infos)
	return exitCode
}

// printLongEntries prints file arguments in long format (no total line).
func printLongEntries(paths []string) int {
	infos := make([]*sys.FileInfo, len(paths))
	exitCode := 0

	for i, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			reportError("cannot access", p, err)
			exitCode = 1
			continue
		}
		infos[i] = fi
	}

	printLongLines(paths, paths, infos)
	return exitCode
}

// printLongLines prints long-format output with aligned columns.
func printLongLines(names, paths []string, infos []*sys.FileInfo) {
	widths := computeLongWidths(infos)
	for i, fi := range infos {
		if fi == nil {
			continue
		}
		printLongLine(names[i], paths[i], fi, widths)
	}
}

// longWidths holds column widths for long-format alignment.
type longWidths struct {
	nlink int
	owner int
	group int
	size  int
}

// computeLongWidths calculates column widths for long-format output.
func computeLongWidths(infos []*sys.FileInfo) longWidths {
	var w longWidths
	for _, fi := range infos {
		if fi == nil {
			continue
		}
		updateWidth(&w.nlink, len(strconv.FormatUint(fi.Nlink, 10)))
		updateWidth(&w.owner, len(lookupUser(fi.Uid)))
		updateWidth(&w.group, len(lookupGroup(fi.Gid)))
		updateWidth(&w.size, len(strconv.FormatInt(fi.Size, 10)))
	}
	return w
}

// updateWidth sets *current to v if v is larger.
func updateWidth(current *int, v int) {
	if v > *current {
		*current = v
	}
}

// printLongLine prints one entry in long format.
// R1.6: fields: permissions nlink owner group size mtime name
func printLongLine(name, path string, fi *sys.FileInfo, w longWidths) {
	perm := permissionString(fi.Mode)
	nlink := format.PadLeft(strconv.FormatUint(fi.Nlink, 10), w.nlink)
	owner := format.PadRight(lookupUser(fi.Uid), w.owner)
	group := format.PadRight(lookupGroup(fi.Gid), w.group)
	size := format.PadLeft(strconv.FormatInt(fi.Size, 10), w.size)
	mtime := formatMtime(fi.ModTime)

	display := name
	// R1.10: symlink display with " -> target".
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err == nil {
			display = name + " -> " + target
		}
	}

	fmt.Printf("%s %s %s %s %s %s %s\n",
		perm, nlink, owner, group, size, mtime, display)
}

// permissionString produces the 10-character permission string.
// R1.6: file type char + owner rwx + group rwx + other rwx
// with setuid/setgid/sticky bit substitution.
func permissionString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	fillRWX(buf[1:4], mode, 6, os.ModeSetuid)
	fillRWX(buf[4:7], mode, 3, os.ModeSetgid)
	fillRWXSticky(buf[7:10], mode)
	return string(buf[:])
}

// fileTypeChar returns the type character for position 0.
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

// fillRWX fills a 3-byte rwx slice for owner or group permissions.
// shift is the bit offset (6 for owner, 3 for group).
// special is ModeSetuid or ModeSetgid.
func fillRWX(buf []byte, mode os.FileMode, shift uint, special os.FileMode) {
	perm := mode.Perm()
	buf[0] = rwChar(perm, 4<<shift, 'r')
	buf[1] = rwChar(perm, 2<<shift, 'w')
	x := rwChar(perm, 1<<shift, 'x')
	if mode&special != 0 {
		if x == 'x' {
			x = 's'
		} else {
			x = 'S'
		}
	}
	buf[2] = x
}

// fillRWXSticky fills the other-rwx with sticky bit handling.
func fillRWXSticky(buf []byte, mode os.FileMode) {
	perm := mode.Perm()
	buf[0] = rwChar(perm, 0o004, 'r')
	buf[1] = rwChar(perm, 0o002, 'w')
	x := rwChar(perm, 0o001, 'x')
	if mode&os.ModeSticky != 0 {
		if x == 'x' {
			x = 't'
		} else {
			x = 'T'
		}
	}
	buf[2] = x
}

// rwChar returns ch if bit is set in perm, '-' otherwise.
func rwChar(perm os.FileMode, bit os.FileMode, ch byte) byte {
	if perm&bit != 0 {
		return ch
	}
	return '-'
}

// lookupUser resolves a UID to a username, falling back to numeric string.
// R1.8: uses os/user.LookupId with numeric fallback.
func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// lookupGroup resolves a GID to a group name, falling back to numeric string.
// R1.8: uses os/user.LookupGroupId with numeric fallback.
func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// sixMonths approximates 6 months for mtime formatting cutoff.
const sixMonths = 6 * 30 * 24 * time.Hour

// formatMtime formats a modification time for long-format display.
// R1.9: recent files show "Jan _2 15:04", older show "Jan _2  2006".
func formatMtime(t time.Time) string {
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// recurseSubdirs handles -R recursive listing for subdirectories.
func recurseSubdirs(dir string, entries []os.DirEntry, cfg lsConfig) int {
	exitCode := 0
	// Collect and sort subdirectory names.
	var subdirs []string
	for _, e := range entries {
		name := e.Name()
		if cfg.filter == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		if name == "." || name == ".." {
			continue
		}
		path := joinPath(dir, name)
		fi, err := sys.Lstat(path)
		if err != nil {
			continue
		}
		if fi.Mode.IsDir() {
			subdirs = append(subdirs, name)
		}
	}
	sort.Strings(subdirs)

	for _, name := range subdirs {
		path := joinPath(dir, name)
		fmt.Println()
		fmt.Printf("%s:\n", path)
		if code := listDir(path, cfg); code != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// reportError prints a diagnostic to stderr matching GNU ls format.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "ls: %s '%s': %s\n", action, path, msg)
}
