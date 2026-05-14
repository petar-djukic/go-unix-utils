// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd008-ls R1.1-R1.8.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).
Sort entries alphabetically if no flags given.

  -1             list one file per line
  -l             use a long listing format
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `ls (go-unix-utils) dev
`

type options struct {
	singleColumn bool
	longFormat   bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, paths := parseArgs(os.Args[1:])
	if len(paths) == 0 {
		paths = []string{"."}
	}
	os.Exit(run(paths, opts))
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var paths []string
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--":
			return opts, append(paths, args...)
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'ls --help' for more information.")
			os.Exit(2)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			parseShortFlags(arg[1:], &opts)
		default:
			paths = append(paths, arg)
		}
	}
	return opts, paths
}

func parseShortFlags(flags string, opts *options) {
	for _, ch := range flags {
		switch ch {
		case '1':
			if !opts.longFormat {
				opts.singleColumn = true
			}
		case 'l':
			opts.longFormat = true
			opts.singleColumn = false
		default:
			fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'ls --help' for more information.")
			os.Exit(2)
		}
	}
}

func run(paths []string, opts options) int {
	exitCode := 0
	files, dirs := classifyPaths(paths, &exitCode)
	needSep := false
	if len(files) > 0 {
		sort.Strings(files)
		if opts.longFormat {
			exitCode = maxCode(exitCode, writeLongFiles(files))
		} else {
			writeEntries(files, opts)
		}
		needSep = true
	}
	showHeader := len(dirs) > 1 || len(files) > 0
	for _, dir := range dirs {
		if needSep {
			fmt.Fprintln(os.Stdout)
		}
		if showHeader {
			fmt.Fprintf(os.Stdout, "%s:\n", dir)
		}
		exitCode = maxCode(exitCode, listDir(dir, opts))
		needSep = true
	}
	return exitCode
}

func classifyPaths(paths []string, exitCode *int) ([]string, []string) {
	var files, dirs []string
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", p, sysError(err))
			*exitCode = 2
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}
	return files, dirs
}

func maxCode(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func listDir(path string, opts options) int {
	names, err := readNames(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': %s\n",
			path, sysError(err))
		return 2
	}
	if opts.longFormat {
		return writeLongDir(path, names)
	}
	writeEntries(names, opts)
	return 0
}

func readNames(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func writeEntries(names []string, opts options) {
	if len(names) == 0 {
		return
	}
	if opts.singleColumn || !sys.IsTerminal(os.Stdout.Fd()) {
		writeLines(names)
	} else {
		writeColumns(names)
	}
}

func writeColumns(names []string) {
	termWidth, err := sys.TerminalWidth()
	if err != nil {
		termWidth = 80
	}
	grid := format.Columns(names, termWidth)
	widths := gridColumnWidths(grid)
	for _, row := range grid {
		writeGridRow(row, widths)
	}
}

func writeLines(names []string) {
	for _, name := range names {
		writeLine(name)
	}
}

func gridColumnWidths(grid [][]string) []int {
	if len(grid) == 0 {
		return nil
	}
	widths := make([]int, len(grid[0]))
	for _, row := range grid {
		for c, cell := range row {
			if w := utf8.RuneCountInString(cell); w > widths[c] {
				widths[c] = w
			}
		}
	}
	return widths
}

func writeGridRow(row []string, widths []int) {
	var b strings.Builder
	for i, cell := range row {
		if i < len(row)-1 {
			b.WriteString(format.PadRight(cell, widths[i]))
			b.WriteString("  ")
		} else {
			b.WriteString(cell)
		}
	}
	writeLine(b.String())
}

type longEntry struct {
	name  string
	info  *sys.FileInfo
	owner string
	group string
}

type longWidths struct {
	nlink int
	owner int
	group int
	size  int
}

func writeLongFiles(paths []string) int {
	entries := make([]longEntry, 0, len(paths))
	code := 0
	for _, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", p, sysError(err))
			code = 2
			continue
		}
		entries = append(entries, newLongEntry(p, fi))
	}
	writeLongEntries(entries)
	return code
}

func writeLongDir(path string, names []string) int {
	entries := make([]longEntry, 0, len(names))
	code := 0
	for _, name := range names {
		full := filepath.Join(path, name)
		fi, err := sys.Lstat(full)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", full, sysError(err))
			code = 2
			continue
		}
		entries = append(entries, newLongEntry(name, fi))
	}
	totalBlocks := int64(0)
	for _, e := range entries {
		totalBlocks += e.info.Blocks
	}
	writeLine(fmt.Sprintf("total %d", totalBlocks/2))
	writeLongEntries(entries)
	return code
}

func newLongEntry(name string, fi *sys.FileInfo) longEntry {
	return longEntry{
		name:  name,
		info:  fi,
		owner: resolveOwner(fi.Uid),
		group: resolveGroup(fi.Gid),
	}
}

func writeLongEntries(entries []longEntry) {
	if len(entries) == 0 {
		return
	}
	w := computeLongWidths(entries)
	for _, e := range entries {
		writeLine(formatLongLine(e, w))
	}
}

func computeLongWidths(entries []longEntry) longWidths {
	var w longWidths
	for _, e := range entries {
		if n := len(strconv.FormatUint(e.info.Nlink, 10)); n > w.nlink {
			w.nlink = n
		}
		if n := len(e.owner); n > w.owner {
			w.owner = n
		}
		if n := len(e.group); n > w.group {
			w.group = n
		}
		if n := len(strconv.FormatInt(e.info.Size, 10)); n > w.size {
			w.size = n
		}
	}
	return w
}

func formatLongLine(e longEntry, w longWidths) string {
	nlink := strconv.FormatUint(e.info.Nlink, 10)
	size := strconv.FormatInt(e.info.Size, 10)
	return permString(e.info.Mode) + " " +
		format.PadLeft(nlink, w.nlink) + " " +
		format.PadRight(e.owner, w.owner) + " " +
		format.PadRight(e.group, w.group) + " " +
		format.PadLeft(size, w.size) + " " +
		formatMtime(e.info.ModTime) + " " +
		e.name
}

func permString(mode os.FileMode) string {
	var b [10]byte
	b[0] = fileTypeChar(mode)
	const rwx = "rwx"
	perm := mode.Perm()
	for i := 0; i < 9; i++ {
		if perm&(1<<uint(8-i)) != 0 {
			b[1+i] = rwx[i%3]
		} else {
			b[1+i] = '-'
		}
	}
	applySpecialBits(mode, &b)
	return string(b[:])
}

func applySpecialBits(mode os.FileMode, b *[10]byte) {
	if mode&os.ModeSetuid != 0 {
		if b[3] == 'x' {
			b[3] = 's'
		} else {
			b[3] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if b[6] == 'x' {
			b[6] = 's'
		} else {
			b[6] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if b[9] == 'x' {
			b[9] = 't'
		} else {
			b[9] = 'T'
		}
	}
}

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

func resolveOwner(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

func formatMtime(t time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	ago := time.Since(t)
	if ago >= 0 && ago < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

func writeLine(s string) {
	if _, err := fmt.Fprintln(os.Stdout, s); err != nil {
		os.Exit(1)
	}
}

func sysError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}
