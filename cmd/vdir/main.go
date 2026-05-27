// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd108-vdir R1.1-R1.4, R2.1-R2.4.
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

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: vdir [OPTION]... [FILE]...
List directory contents.

  -a             do not ignore entries starting with .
  -A             do not list implied . and ..
  -C             list entries by columns
  -d             list directories themselves, not their contents
  -F             append indicator (one of */=>@|) to entries
  -h             with -l and/or -s, print human readable sizes
  -i             print the index number of each file
  -l             use a long listing format
  -n             like -l, but list numeric user and group IDs
  -R             list subdirectories recursively
  -r             reverse order while sorting
  -s             print the allocated size of each file, in blocks
  -S             sort by file size, largest first
  -t             sort by modification time, newest first
  -U             do not sort; list entries in directory order
  -v             natural sort of (version) numbers within text
  -x             list entries by lines instead of by columns
  -1             list one file per line
      --color[=WHEN]  colorize the output; WHEN can be 'always'
                         (default if omitted), 'auto', or 'never'
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `vdir (go-unix-utils) dev
`

type options struct {
	singleColumn   bool
	longFormat     bool
	forceColumns   bool
	horizontalSort bool
	showAll        bool
	showAlmostAll  bool
	dirAsEntry     bool
	sortByTime     bool
	sortBySize     bool
	reverseSort    bool
	unsorted       bool
	versionSort    bool
	showInode      bool
	showBlocks     bool
	numericIds     bool
	humanReadable  bool
	classify       bool
	recursive      bool
	colorMode      string
}

type fileEntry struct {
	name string
	info *sys.FileInfo
	link string
}

type longRow struct {
	mode, nlink, owner, group, size, time, name string
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, paths := parseArgs(os.Args[1:])
	os.Exit(run(paths, opts))
}

func parseArgs(args []string) (options, []string) {
	opts := options{longFormat: true}
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
		case arg == "--color":
			opts.colorMode = "always"
		case strings.HasPrefix(arg, "--color="):
			opts.colorMode = parseColorValue(arg)
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "vdir: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'vdir --help' for more information.")
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
			opts.forceColumns = false
			opts.horizontalSort = false
			if !opts.longFormat {
				opts.singleColumn = true
			}
		case 'l', 'g', 'n':
			if ch == 'n' {
				opts.numericIds = true
			}
			opts.longFormat = true
			opts.singleColumn = false
			opts.forceColumns = false
			opts.horizontalSort = false
		case 'C', 'x':
			opts.forceColumns = true
			opts.horizontalSort = ch == 'x'
			opts.longFormat = false
			opts.singleColumn = false
		case 'a':
			opts.showAll = true
			opts.showAlmostAll = false
		case 'A':
			opts.showAlmostAll = true
			opts.showAll = false
		case 'b', 'B', 'c', 'G', 'H', 'k', 'L', 'm', 'N',
			'o', 'p', 'q', 'Q', 'u', 'X', 'Z':
		case 'd':
			opts.dirAsEntry = true
		case 'f':
			opts.showAll = true
			opts.showAlmostAll = false
			opts.unsorted = true
			opts.sortByTime = false
			opts.sortBySize = false
			opts.versionSort = false
		case 'F':
			opts.classify = true
		case 'h':
			opts.humanReadable = true
		case 'i':
			opts.showInode = true
		case 'R':
			opts.recursive = true
		case 'r':
			opts.reverseSort = true
		case 's':
			opts.showBlocks = true
		case 't', 'S', 'U', 'v':
			opts.sortByTime = ch == 't'
			opts.sortBySize = ch == 'S'
			opts.unsorted = ch == 'U'
			opts.versionSort = ch == 'v'
		default:
			fmt.Fprintf(os.Stderr, "vdir: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'vdir --help' for more information.")
			os.Exit(2)
		}
	}
}

func parseColorValue(arg string) string {
	val := arg[len("--color="):]
	switch val {
	case "always", "auto", "never":
		return val
	default:
		fmt.Fprintf(os.Stderr, "vdir: invalid argument '%s' for '--color'\n", val)
		fmt.Fprintln(os.Stderr, "Try 'vdir --help' for more information.")
		os.Exit(2)
		return ""
	}
}

func classifyPaths(paths []string, opts options) ([]fileEntry, []string, int) {
	var files []fileEntry
	var dirs []string
	exitCode := 0
	for _, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vdir: cannot access '%s': %s\n", p, sysErrMsg(err))
			exitCode = 2
			continue
		}
		if fi.Mode.IsDir() && !opts.dirAsEntry {
			dirs = append(dirs, p)
		} else {
			link := ""
			if fi.Mode&os.ModeSymlink != 0 {
				link, _ = os.Readlink(p)
			}
			files = append(files, fileEntry{name: p, info: fi, link: link})
		}
	}
	return files, dirs, exitCode
}

func run(paths []string, opts options) int {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, dirs, exitCode := classifyPaths(paths, opts)
	showHeaders := len(dirs) > 1 || len(files) > 0 || opts.recursive
	printed := false
	if len(files) > 0 {
		sortEntries(files, opts)
		writeOutput(files, -1, opts)
		printed = true
	}
	for _, d := range dirs {
		if printed {
			fmt.Println()
		}
		if showHeaders {
			fmt.Printf("%s:\n", d)
		}
		entries, totalBlocks, err := listDir(d, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vdir: cannot open directory '%s': %s\n", d, sysErrMsg(err))
			exitCode = 1
			printed = true
			continue
		}
		writeOutput(entries, totalBlocks, opts)
		printed = true
	}
	return exitCode
}

func listDir(path string, opts options) ([]fileEntry, int64, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]fileEntry, 0, len(dirEntries)+2)
	if opts.showAll {
		if fi, e := sys.Lstat(path); e == nil {
			entries = append(entries, fileEntry{name: ".", info: fi})
		}
		if fi, e := sys.Lstat(filepath.Join(path, "..")); e == nil {
			entries = append(entries, fileEntry{name: "..", info: fi})
		}
	}
	for _, de := range dirEntries {
		name := de.Name()
		if len(name) > 0 && name[0] == '.' && !opts.showAll && !opts.showAlmostAll {
			continue
		}
		fullPath := filepath.Join(path, name)
		fi, err := sys.Lstat(fullPath)
		if err != nil {
			continue
		}
		link := ""
		if fi.Mode&os.ModeSymlink != 0 {
			link, _ = os.Readlink(fullPath)
		}
		entries = append(entries, fileEntry{name: name, info: fi, link: link})
	}
	sortEntries(entries, opts)
	var totalBlocks int64
	for _, e := range entries {
		totalBlocks += e.info.Blocks
	}
	return entries, totalBlocks / 2, nil
}

func sortEntries(entries []fileEntry, opts options) {
	if opts.unsorted {
		return
	}
	less := nameLess
	if opts.sortByTime {
		less = timeLess
	} else if opts.sortBySize {
		less = sizeLess
	}
	if opts.reverseSort {
		fwd := less
		less = func(a, b fileEntry) bool { return fwd(b, a) }
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return less(entries[i], entries[j])
	})
}

func nameLess(a, b fileEntry) bool { return a.name < b.name }
func timeLess(a, b fileEntry) bool { return a.info.ModTime.After(b.info.ModTime) }
func sizeLess(a, b fileEntry) bool { return a.info.Size > b.info.Size }

func writeOutput(entries []fileEntry, totalBlocks int64, opts options) {
	if opts.longFormat {
		writeLong(entries, totalBlocks, opts)
		return
	}
	for _, e := range entries {
		fmt.Println(escapeC(e.name))
	}
}

func writeLong(entries []fileEntry, totalBlocks int64, opts options) {
	if totalBlocks >= 0 {
		fmt.Printf("total %d\n", totalBlocks)
	}
	if len(entries) == 0 {
		return
	}
	rows := make([]longRow, len(entries))
	var wN, wO, wG, wS int
	for i, e := range entries {
		rows[i] = buildRow(e, opts)
		wN = max(wN, len(rows[i].nlink))
		wO = max(wO, len(rows[i].owner))
		wG = max(wG, len(rows[i].group))
		wS = max(wS, len(rows[i].size))
	}
	for _, r := range rows {
		fmt.Printf("%s %s %s %s %s %s %s\n", r.mode,
			format.PadLeft(r.nlink, wN), format.PadRight(r.owner, wO),
			format.PadRight(r.group, wG), format.PadLeft(r.size, wS),
			r.time, r.name)
	}
}

func buildRow(e fileEntry, opts options) longRow {
	fi := e.info
	owner, group := lookupName(fi.Uid, true), lookupName(fi.Gid, false)
	if opts.numericIds {
		owner = strconv.FormatUint(uint64(fi.Uid), 10)
		group = strconv.FormatUint(uint64(fi.Gid), 10)
	}
	size := strconv.FormatInt(fi.Size, 10)
	if opts.humanReadable {
		size = format.HumanSize(fi.Size, format.HumanSizeOpts{Binary: true})
	}
	name := escapeC(e.name)
	if e.link != "" {
		name += " -> " + escapeC(e.link)
	}
	return longRow{
		mode:  formatMode(fi.Mode),
		nlink: strconv.FormatUint(fi.Nlink, 10),
		owner: owner, group: group,
		size: size, time: formatTime(fi.ModTime),
		name: name,
	}
}

func formatMode(mode os.FileMode) string {
	var buf [10]byte
	switch {
	case mode&os.ModeDir != 0:
		buf[0] = 'd'
	case mode&os.ModeSymlink != 0:
		buf[0] = 'l'
	case mode&os.ModeNamedPipe != 0:
		buf[0] = 'p'
	case mode&os.ModeSocket != 0:
		buf[0] = 's'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		buf[0] = 'c'
	case mode&os.ModeDevice != 0:
		buf[0] = 'b'
	default:
		buf[0] = '-'
	}
	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if mode&(1<<uint(8-i)) != 0 {
			buf[1+i] = byte(c)
		} else {
			buf[1+i] = '-'
		}
	}
	setSpecial := func(pos int, withX, withoutX byte) {
		if buf[pos] == 'x' {
			buf[pos] = withX
		} else {
			buf[pos] = withoutX
		}
	}
	if mode&os.ModeSetuid != 0 {
		setSpecial(3, 's', 'S')
	}
	if mode&os.ModeSetgid != 0 {
		setSpecial(6, 's', 'S')
	}
	if mode&os.ModeSticky != 0 {
		setSpecial(9, 't', 'T')
	}
	return string(buf[:])
}

func formatTime(t time.Time) string {
	now := time.Now()
	sixMonths := 365 * 24 * time.Hour / 2
	if t.After(now.Add(-sixMonths)) && !t.After(now) {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

func lookupName(id uint32, isUser bool) string {
	s := strconv.FormatUint(uint64(id), 10)
	if isUser {
		u, err := user.LookupId(s)
		if err == nil {
			return u.Username
		}
	} else {
		g, err := user.LookupGroupId(s)
		if err == nil {
			return g.Name
		}
	}
	return s
}

func escapeC(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString("\\\\")
		case '\a':
			b.WriteString("\\a")
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\v':
			b.WriteString("\\v")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		case ' ':
			b.WriteString("\\ ")
		default:
			if c < 0x20 || c >= 0x7f {
				b.WriteByte('\\')
				b.WriteByte('0' + c>>6)
				b.WriteByte('0' + (c>>3)&7)
				b.WriteByte('0' + c&7)
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

func sysErrMsg(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return capitalize(pe.Err.Error())
	}
	return capitalize(err.Error())
}

func capitalize(s string) string {
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}
