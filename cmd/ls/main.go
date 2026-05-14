// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd008-ls R1.1-R1.14, R2.1-R2.15, R3.1-R3.15, R4.1-R4.9.
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

const versionText = `ls (go-unix-utils) dev
`

type options struct {
	singleColumn  bool
	longFormat    bool
	forceColumns  bool
	horizontalSort bool
	showAll       bool
	showAlmostAll bool
	dirAsEntry    bool
	sortByTime    bool
	sortBySize    bool
	reverseSort   bool
	unsorted      bool
	versionSort   bool
	showInode     bool
	showBlocks    bool
	numericIds    bool
	humanReadable bool
	classify      bool
	recursive     bool
	colorMode     string
}

var termWidth int

func main() {
	sys.InstallSIGPIPEHandler()
	termWidth = queryTermWidth()
	sys.OnTerminalResize(func(w int) { termWidth = w })
	opts, paths := parseArgs(os.Args[1:])
	if len(paths) == 0 {
		paths = []string{"."}
	}
	os.Exit(run(paths, opts))
}

func queryTermWidth() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return 80
	}
	return w
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
		case arg == "--color":
			opts.colorMode = "always"
		case strings.HasPrefix(arg, "--color="):
			opts.colorMode = parseColorValue(arg)
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
			opts.forceColumns = false
			opts.horizontalSort = false
			if !opts.longFormat {
				opts.singleColumn = true
			}
		case 'l':
			opts.longFormat = true
			opts.singleColumn = false
			opts.forceColumns = false
			opts.horizontalSort = false
		case 'C':
			opts.forceColumns = true
			opts.horizontalSort = false
			opts.longFormat = false
			opts.singleColumn = false
		case 'x':
			opts.forceColumns = true
			opts.horizontalSort = true
			opts.longFormat = false
			opts.singleColumn = false
		case 'a':
			opts.showAll = true
			opts.showAlmostAll = false
		case 'A':
			opts.showAlmostAll = true
			opts.showAll = false
		case 'd':
			opts.dirAsEntry = true
		case 'F':
			opts.classify = true
		case 'h':
			opts.humanReadable = true
		case 'i':
			opts.showInode = true
		case 'n':
			opts.numericIds = true
			opts.longFormat = true
			opts.singleColumn = false
			opts.forceColumns = false
			opts.horizontalSort = false
		case 'R':
			opts.recursive = true
		case 'r':
			opts.reverseSort = true
		case 's':
			opts.showBlocks = true
		case 't':
			opts.sortByTime = true
			opts.sortBySize = false
			opts.unsorted = false
			opts.versionSort = false
		case 'S':
			opts.sortBySize = true
			opts.sortByTime = false
			opts.unsorted = false
			opts.versionSort = false
		case 'U':
			opts.unsorted = true
			opts.sortByTime = false
			opts.sortBySize = false
			opts.versionSort = false
		case 'v':
			opts.versionSort = true
			opts.sortByTime = false
			opts.sortBySize = false
			opts.unsorted = false
		default:
			fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'ls --help' for more information.")
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
		fmt.Fprintf(os.Stderr,
			"ls: invalid argument '%s' for '--color'\n", val)
		fmt.Fprintln(os.Stderr,
			"Try 'ls --help' for more information.")
		os.Exit(2)
		return ""
	}
}

func setupColor(opts options) {
	switch opts.colorMode {
	case "always":
		format.SetColorEnabled(true)
	case "auto":
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	default:
		format.SetColorEnabled(false)
	}
}

func run(paths []string, opts options) int {
	setupColor(opts)
	exitCode := 0
	files, dirs := classifyPaths(paths, &exitCode, opts.dirAsEntry)
	needSep := false
	if len(files) > 0 {
		sortEntries(files, "", opts)
		if opts.longFormat {
			exitCode = maxCode(exitCode, writeLongFiles(files, opts))
		} else {
			display := buildDisplayNames(files, "", opts)
			writeEntries(display, opts)
		}
		needSep = true
	}
	showHeader := len(dirs) > 1 || len(files) > 0 || opts.recursive
	sortEntries(dirs, "", opts)
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

func classifyPaths(paths []string, exitCode *int, dirAsEntry bool) ([]string, []string) {
	var files, dirs []string
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", p, sysError(err))
			*exitCode = 1
			continue
		}
		if info.IsDir() && !dirAsEntry {
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
	names, err := readNames(path, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': %s\n",
			path, sysError(err))
		return 1
	}
	code := 0
	if opts.longFormat {
		code = writeLongDir(path, names, opts)
	} else {
		if opts.showBlocks {
			writeTotalLine(path, names, opts)
		}
		display := buildDisplayNames(names, path, opts)
		writeEntries(display, opts)
	}
	if opts.recursive {
		code = maxCode(code, recurseSubdirs(path, names, opts))
	}
	return code
}

func recurseSubdirs(path string, names []string, opts options) int {
	code := 0
	for _, name := range names {
		if name == "." || name == ".." {
			continue
		}
		full := filepath.Join(path, name)
		fi, err := os.Lstat(full)
		if err != nil || !fi.IsDir() {
			continue
		}
		fmt.Fprintln(os.Stdout)
		fmt.Fprintf(os.Stdout, "%s:\n", full)
		code = maxCode(code, listDir(full, opts))
	}
	return code
}

func readNames(path string, opts options) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	if opts.showAll {
		names = append(names, ".", "..")
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && !opts.showAll && !opts.showAlmostAll {
			continue
		}
		names = append(names, e.Name())
	}
	sortEntries(names, path, opts)
	return names, nil
}

func sortEntries(names []string, basePath string, opts options) {
	if opts.unsorted {
		return
	}
	if opts.versionSort {
		sort.SliceStable(names, func(i, j int) bool {
			return strverscmp(names[i], names[j]) < 0
		})
		if opts.reverseSort {
			reverseSlice(names)
		}
		return
	}
	if !opts.sortByTime && !opts.sortBySize {
		sort.Strings(names)
		if opts.reverseSort {
			reverseSlice(names)
		}
		return
	}
	type entry struct {
		name    string
		modTime time.Time
		size    int64
	}
	entries := make([]entry, len(names))
	for i, name := range names {
		entries[i].name = name
		p := name
		if basePath != "" {
			p = filepath.Join(basePath, name)
		}
		if fi, err := sys.Lstat(p); err == nil {
			entries[i].modTime = fi.ModTime
			entries[i].size = fi.Size
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if opts.sortByTime {
			if !entries[i].modTime.Equal(entries[j].modTime) {
				return entries[i].modTime.After(entries[j].modTime)
			}
			return entries[i].name < entries[j].name
		}
		if entries[i].size != entries[j].size {
			return entries[i].size > entries[j].size
		}
		return entries[i].name < entries[j].name
	})
	for i, e := range entries {
		names[i] = e.name
	}
	if opts.reverseSort {
		reverseSlice(names)
	}
}

type entryInfo struct {
	inode  uint64
	blocks int64
	mode   os.FileMode
}

func statDisplayEntries(names []string, basePath string, opts options) ([]entryInfo, int, int) {
	infos := make([]entryInfo, len(names))
	var inodeW, blocksW int
	for i, name := range names {
		p := name
		if basePath != "" {
			p = filepath.Join(basePath, name)
		}
		fi, err := sys.Lstat(p)
		if err != nil {
			continue
		}
		infos[i] = entryInfo{fi.Ino, fi.Blocks / 2, fi.Mode}
		if n := len(strconv.FormatUint(fi.Ino, 10)); n > inodeW {
			inodeW = n
		}
		bstr := formatBlocks(infos[i].blocks, opts.humanReadable)
		if n := len(bstr); n > blocksW {
			blocksW = n
		}
	}
	return infos, inodeW, blocksW
}

func buildDisplayNames(names []string, basePath string, opts options) []string {
	needPrefix := opts.showInode || opts.showBlocks
	needColor := format.FileTypeColor(os.ModeDir) != ""
	if !needPrefix && !needColor && !opts.classify {
		return names
	}
	infos, inodeW, blocksW := statDisplayEntries(names, basePath, opts)
	result := make([]string, len(names))
	for i, name := range names {
		var b strings.Builder
		if opts.showInode {
			b.WriteString(format.PadLeft(strconv.FormatUint(infos[i].inode, 10), inodeW))
			b.WriteByte(' ')
		}
		if opts.showBlocks {
			bstr := formatBlocks(infos[i].blocks, opts.humanReadable)
			b.WriteString(format.PadLeft(bstr, blocksW))
			b.WriteByte(' ')
		}
		b.WriteString(colorEntry(name, infos[i].mode, opts.classify))
		result[i] = b.String()
	}
	return result
}

func colorEntry(name string, mode os.FileMode, classify bool) string {
	indicator := ""
	if classify {
		indicator = classifyIndicator(mode)
	}
	c := format.FileTypeColor(mode)
	if c == "" {
		return name + indicator
	}
	return c + name + format.Reset() + indicator
}

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
	case mode.IsRegular() && mode&0111 != 0:
		return "*"
	default:
		return ""
	}
}

func writeEntries(names []string, opts options) {
	if len(names) == 0 {
		return
	}
	if opts.forceColumns {
		if opts.horizontalSort {
			writeHorizontalColumns(names, 0)
		} else {
			writeColumnsWidth(names, 0)
		}
	} else if opts.singleColumn || !sys.IsTerminal(os.Stdout.Fd()) {
		writeLines(names)
	} else {
		writeColumnsWidth(names, 0)
	}
}

func writeColumnsWidth(names []string, width int) {
	if width <= 0 {
		width = termWidth
	}
	grid := format.Columns(names, width)
	widths := gridColumnWidths(grid)
	for _, row := range grid {
		writeGridRow(row, widths)
	}
}

func writeHorizontalColumns(names []string, width int) {
	if width <= 0 {
		width = termWidth
	}
	numCols := findHorizontalMaxCols(names, width)
	rows := (len(names) + numCols - 1) / numCols
	colWidths := make([]int, numCols)
	for c := 0; c < numCols; c++ {
		for r := 0; r < rows; r++ {
			idx := r*numCols + c
			if idx >= len(names) {
				break
			}
			if w := utf8.RuneCountInString(names[idx]); w > colWidths[c] {
				colWidths[c] = w
			}
		}
	}
	for r := 0; r < rows; r++ {
		var row []string
		for c := 0; c < numCols; c++ {
			idx := r*numCols + c
			if idx >= len(names) {
				break
			}
			row = append(row, names[idx])
		}
		writeGridRow(row, colWidths)
	}
}

func findHorizontalMaxCols(entries []string, termWidth int) int {
	n := len(entries)
	for cols := n; cols > 1; cols-- {
		if horizontalGridFits(entries, cols, termWidth) {
			return cols
		}
	}
	return 1
}

func horizontalGridFits(entries []string, cols, termWidth int) bool {
	rows := (len(entries) + cols - 1) / cols
	colWidths := make([]int, cols)
	for c := 0; c < cols; c++ {
		for r := 0; r < rows; r++ {
			idx := r*cols + c
			if idx >= len(entries) {
				break
			}
			if w := utf8.RuneCountInString(entries[idx]); w > colWidths[c] {
				colWidths[c] = w
			}
		}
	}
	total := 0
	for c := 0; c < cols; c++ {
		if c < cols-1 {
			total += colWidths[c] + 2
		} else {
			total += colWidths[c]
		}
		if total > termWidth {
			return false
		}
	}
	return true
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
	path  string
	info  *sys.FileInfo
	owner string
	group string
}

type longWidths struct {
	inode  int
	blocks int
	nlink  int
	owner  int
	group  int
	size   int
}

func writeLongFiles(paths []string, opts options) int {
	entries := make([]longEntry, 0, len(paths))
	code := 0
	for _, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", p, sysError(err))
			code = 1
			continue
		}
		entries = append(entries, newLongEntry(p, p, fi, opts))
	}
	writeLongEntries(entries, opts)
	return code
}

func writeTotalLine(path string, names []string, opts options) {
	total := int64(0)
	for _, name := range names {
		p := filepath.Join(path, name)
		fi, err := sys.Lstat(p)
		if err != nil {
			continue
		}
		total += fi.Blocks
	}
	writeLine(fmt.Sprintf("total %s", formatBlocks(total/2, opts.humanReadable)))
}

func writeLongDir(path string, names []string, opts options) int {
	entries := make([]longEntry, 0, len(names))
	code := 0
	for _, name := range names {
		full := filepath.Join(path, name)
		fi, err := sys.Lstat(full)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", full, sysError(err))
			code = 1
			continue
		}
		entries = append(entries, newLongEntry(name, full, fi, opts))
	}
	totalBlocks := int64(0)
	for _, e := range entries {
		totalBlocks += e.info.Blocks
	}
	total := formatBlocks(totalBlocks/2, opts.humanReadable)
	writeLine(fmt.Sprintf("total %s", total))
	writeLongEntries(entries, opts)
	return code
}

func newLongEntry(name, path string, fi *sys.FileInfo, opts options) longEntry {
	owner := resolveOwner(fi.Uid)
	group := resolveGroup(fi.Gid)
	if opts.numericIds {
		owner = strconv.FormatUint(uint64(fi.Uid), 10)
		group = strconv.FormatUint(uint64(fi.Gid), 10)
	}
	return longEntry{
		name:  name,
		path:  path,
		info:  fi,
		owner: owner,
		group: group,
	}
}

func writeLongEntries(entries []longEntry, opts options) {
	if len(entries) == 0 {
		return
	}
	w := computeLongWidths(entries, opts)
	for _, e := range entries {
		writeLine(formatLongLine(e, w, opts))
	}
}

func computeLongWidths(entries []longEntry, opts options) longWidths {
	var w longWidths
	for _, e := range entries {
		if opts.showInode {
			if n := len(strconv.FormatUint(e.info.Ino, 10)); n > w.inode {
				w.inode = n
			}
		}
		if opts.showBlocks {
			bstr := formatBlocks(e.info.Blocks/2, opts.humanReadable)
			if n := len(bstr); n > w.blocks {
				w.blocks = n
			}
		}
		if n := len(strconv.FormatUint(e.info.Nlink, 10)); n > w.nlink {
			w.nlink = n
		}
		if n := len(e.owner); n > w.owner {
			w.owner = n
		}
		if n := len(e.group); n > w.group {
			w.group = n
		}
		sstr := formatSize(e.info.Size, opts.humanReadable)
		if n := len(sstr); n > w.size {
			w.size = n
		}
	}
	return w
}

func formatLongLine(e longEntry, w longWidths, opts options) string {
	var prefix string
	if opts.showInode {
		prefix += format.PadLeft(strconv.FormatUint(e.info.Ino, 10), w.inode) + " "
	}
	if opts.showBlocks {
		bstr := formatBlocks(e.info.Blocks/2, opts.humanReadable)
		prefix += format.PadLeft(bstr, w.blocks) + " "
	}
	nlink := strconv.FormatUint(e.info.Nlink, 10)
	size := formatSize(e.info.Size, opts.humanReadable)
	classifyName := opts.classify && e.info.Mode&os.ModeSymlink == 0
	name := colorEntry(e.name, e.info.Mode, classifyName)
	if e.info.Mode&os.ModeSymlink != 0 {
		if target, err := os.Readlink(e.path); err == nil {
			name += " -> " + target
		}
	}
	return prefix +
		permString(e.info.Mode) + " " +
		format.PadLeft(nlink, w.nlink) + " " +
		format.PadRight(e.owner, w.owner) + " " +
		format.PadRight(e.group, w.group) + " " +
		format.PadLeft(size, w.size) + " " +
		formatMtime(e.info.ModTime) + " " +
		name
}

func formatBlocks(blocks int64, human bool) string {
	if !human {
		return strconv.FormatInt(blocks, 10)
	}
	return format.HumanSize(blocks*1024, format.HumanSizeOpts{Binary: true})
}

func formatSize(size int64, human bool) string {
	if !human {
		return strconv.FormatInt(size, 10)
	}
	return format.HumanSize(size, format.HumanSizeOpts{Binary: true})
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
