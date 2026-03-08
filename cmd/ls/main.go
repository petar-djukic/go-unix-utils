// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ls utility for listing directory contents.
//
// Implements prd008-ls: output formats (R1), filtering/sorting/metadata (R2),
// display features (R3), exit codes and signal handling (R4).
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
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// formatMode identifies the output format.
type formatMode int

const (
	formatDefault formatMode = iota // auto-detect based on TTY
	formatLong                      // -l
	formatSingle                    // -1
	formatColumns                   // -C
	formatAcross                    // -x
)

// filterMode identifies the dotfile filter.
type filterMode int

const (
	filterNormal    filterMode = iota // hide dotfiles
	filterAll                         // -a: show all including . and ..
	filterAlmostAll                   // -A: show dotfiles except . and ..
)

// sortMode identifies the sort strategy.
type sortMode int

const (
	sortName    sortMode = iota // default: C locale lexicographic
	sortTime                    // -t: modification time
	sortSize                    // -S: file size
	sortNone                    // -U: directory order
	sortVersion                 // -v: version sort
)

// flags holds the parsed command-line options.
type flags struct {
	format      formatMode
	filter      filterMode
	sortBy      sortMode
	reverse     bool   // -r
	recursive   bool   // -R
	dirOnly     bool   // -d
	humanSize   bool   // -h
	classify    bool   // -F
	appendSlash bool   // -p
	showInode   bool   // -i
	showBlocks  bool   // -s
	numericIDs  bool   // -n
	colorMode   string // "auto", "always", "never"
}

// exitCode tracks the worst exit code seen.
var exitCode int

func main() {
	sys.InstallSIGPIPEHandler()

	f, paths := parseArgs(os.Args[1:])

	// R3.3: Set color mode based on --color flag.
	switch f.colorMode {
	case "always":
		format.SetColorEnabled(true)
	case "never":
		format.SetColorEnabled(false)
	case "auto":
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Separate files from directories.
	var files []string
	var dirs []string
	for _, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': No such file or directory\n", p)
			exitCode = 1
			continue
		}
		if fi.Mode.IsDir() && !f.dirOnly {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}

	// List individual files first.
	if len(files) > 0 {
		listFiles(files, f)
	}

	// List directories.
	showHeaders := len(files) > 0 || len(dirs) > 1 || f.recursive
	for i, d := range dirs {
		if showHeaders {
			if i > 0 || len(files) > 0 {
				fmt.Println()
			}
			fmt.Printf("%s:\n", d)
		}
		listDir(d, f)
	}

	// R4.2/R4.3: Exit 2 when all arguments failed (no files or dirs survived).
	if exitCode > 0 && len(files) == 0 && len(dirs) == 0 {
		exitCode = 2
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and paths.
func parseArgs(args []string) (flags, []string) {
	var f flags
	f.colorMode = "auto"
	var paths []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			paths = append(paths, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// Long flags.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--color" || arg == "--color=always":
				f.colorMode = "always"
			case arg == "--color=auto":
				f.colorMode = "auto"
			case arg == "--color=never":
				f.colorMode = "never"
			case strings.HasPrefix(arg, "--color="):
				fmt.Fprintf(os.Stderr, "ls: invalid argument '%s' for '--color'\n", arg[len("--color="):])
				os.Exit(2)
			default:
				fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
				os.Exit(2)
			}
			continue
		}

		// Short flags.
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'l':
					f.format = formatLong
				case '1':
					f.format = formatSingle
				case 'C':
					f.format = formatColumns
				case 'x':
					f.format = formatAcross
				case 'a':
					f.filter = filterAll
				case 'A':
					f.filter = filterAlmostAll
				case 'd':
					f.dirOnly = true
				case 'r':
					f.reverse = true
				case 't':
					f.sortBy = sortTime
				case 'S':
					f.sortBy = sortSize
				case 'U':
					f.sortBy = sortNone
				case 'v':
					f.sortBy = sortVersion
				case 'R':
					f.recursive = true
				case 'h':
					f.humanSize = true
				case 'F':
					f.classify = true
				case 'p':
					f.appendSlash = true
				case 'i':
					f.showInode = true
				case 's':
					f.showBlocks = true
				case 'n':
					f.numericIDs = true
					f.format = formatLong
				default:
					fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
					os.Exit(2)
				}
			}
			continue
		}

		paths = append(paths, arg)
	}

	return f, paths
}

// entry holds a directory entry's name and metadata.
type entry struct {
	name string
	info *sys.FileInfo
	path string // full path for stat
}

// listFiles lists individual file arguments (not directory contents).
func listFiles(paths []string, f flags) {
	var entries []entry
	for _, p := range paths {
		fi, err := sys.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': No such file or directory\n", p)
			exitCode = 1
			continue
		}
		entries = append(entries, entry{name: p, info: fi, path: p})
	}

	sortEntries(entries, f)
	printEntries(entries, f)
}

// listDir lists the contents of a single directory.
func listDir(dir string, f flags) {
	dirFile, err := os.Open(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': Permission denied\n", dir)
		exitCode = 1
		return
	}
	defer dirFile.Close()

	names, err := dirFile.Readdirnames(-1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: reading directory '%s': %v\n", dir, err)
		exitCode = 1
	}

	// Apply filter.
	var filtered []string
	switch f.filter {
	case filterAll:
		filtered = append(filtered, ".", "..")
		filtered = append(filtered, names...)
	case filterAlmostAll:
		filtered = append(filtered, names...)
	default:
		for _, n := range names {
			if n[0] != '.' {
				filtered = append(filtered, n)
			}
		}
	}

	// Stat each entry.
	var entries []entry
	for _, n := range filtered {
		p := filepath.Join(dir, n)
		fi, err := sys.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %v\n", p, err)
			exitCode = 1
			continue
		}
		entries = append(entries, entry{name: n, info: fi, path: p})
	}

	sortEntries(entries, f)

	// R1.10, R2.13: Print total block count for long format or when -s is active.
	if f.format == formatLong || f.showBlocks {
		var totalBlocks int64
		for _, e := range entries {
			totalBlocks += e.info.Blocks
		}
		totalK := totalBlocks / 2
		if f.humanSize {
			// R3.6, R3.7: Human-readable total.
			fmt.Printf("total %s\n", format.HumanSize(totalK*1024, format.HumanSizeOpts{Binary: true}))
		} else {
			fmt.Printf("total %d\n", totalK)
		}
	}

	printEntries(entries, f)

	// R3.11: Recursive listing.
	if f.recursive {
		for _, e := range entries {
			if e.info.Mode.IsDir() && e.name != "." && e.name != ".." {
				// R3.13: Do not follow symlinks to directories.
				if e.info.Mode&os.ModeSymlink != 0 {
					continue
				}
				fmt.Printf("\n%s:\n", e.path)
				listDir(e.path, f)
			}
		}
	}
}

// sortEntries sorts entries according to the flags.
func sortEntries(entries []entry, f flags) {
	if f.sortBy == sortNone {
		return
	}

	sort.SliceStable(entries, func(i, j int) bool {
		var less bool
		switch f.sortBy {
		case sortTime:
			if entries[i].info.ModTime.Equal(entries[j].info.ModTime) {
				less = entries[i].name < entries[j].name
			} else {
				less = entries[i].info.ModTime.After(entries[j].info.ModTime)
			}
		case sortSize:
			if entries[i].info.Size == entries[j].info.Size {
				less = entries[i].name < entries[j].name
			} else {
				less = entries[i].info.Size > entries[j].info.Size
			}
		case sortVersion:
			less = versionLess(entries[i].name, entries[j].name)
		default:
			less = entries[i].name < entries[j].name
		}
		if f.reverse {
			return !less
		}
		return less
	})
}

// versionLess compares two strings using natural version sort (strverscmp).
// R2.9: Runs of digits are compared numerically.
func versionLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigit(ca) && isDigit(cb) {
			na, ea := extractNumber(a, ai)
			nb, eb := extractNumber(b, bi)
			if na != nb {
				return na < nb
			}
			ai = ea
			bi = eb
		} else {
			if ca != cb {
				return ca < cb
			}
			ai++
			bi++
		}
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func extractNumber(s string, start int) (int, int) {
	end := start
	for end < len(s) && isDigit(s[end]) {
		end++
	}
	n, _ := strconv.Atoi(s[start:end])
	return n, end
}

// printEntries prints entries in the appropriate format.
func printEntries(entries []entry, f flags) {
	if len(entries) == 0 {
		return
	}

	switch f.format {
	case formatLong:
		printLong(entries, f)
	case formatSingle:
		printSingleColumn(entries, f)
	case formatColumns:
		printMultiColumn(entries, f, false)
	case formatAcross:
		printMultiColumn(entries, f, true)
	default:
		if sys.IsTerminal(os.Stdout.Fd()) {
			printMultiColumn(entries, f, false)
		} else {
			printSingleColumn(entries, f)
		}
	}
}

// printSingleColumn prints one entry per line.
func printSingleColumn(entries []entry, f flags) {
	inodeWidth, blockWidth := metaWidths(entries, f)

	for _, e := range entries {
		printMetaPrefix(e, f, inodeWidth, blockWidth)
		fmt.Print(colorize(displayName(e, f), e.info.Mode))
		fmt.Print(classifyChar(e, f))
		fmt.Println()
	}
}

// printMultiColumn prints entries in multi-column layout.
func printMultiColumn(entries []entry, f flags, across bool) {
	inodeWidth, blockWidth := metaWidths(entries, f)
	var displayEntries []string
	for _, e := range entries {
		var prefix string
		if f.showInode {
			prefix += format.PadLeft(strconv.FormatUint(e.info.Ino, 10), inodeWidth) + " "
		}
		if f.showBlocks {
			bk := e.info.Blocks / 2
			if f.humanSize {
				prefix += format.PadLeft(humanSizeBlocks(bk), blockWidth) + " "
			} else {
				prefix += format.PadLeft(strconv.FormatInt(bk, 10), blockWidth) + " "
			}
		}
		name := colorize(displayName(e, f), e.info.Mode) + classifyChar(e, f)
		displayEntries = append(displayEntries, prefix+name)
	}

	termWidth := 80
	if w, err := sys.TerminalWidth(); err == nil {
		termWidth = w
	}

	if across {
		printAcross(displayEntries, termWidth)
	} else {
		rows := format.Columns(displayEntries, termWidth)
		if rows == nil {
			return
		}
		numCols := len(rows[0])
		colWidths := make([]int, numCols)
		for _, row := range rows {
			for c, cell := range row {
				w := utf8.RuneCountInString(cell)
				if w > colWidths[c] {
					colWidths[c] = w
				}
			}
		}
		for _, row := range rows {
			for c, cell := range row {
				if c < len(row)-1 {
					fmt.Print(format.PadRight(cell, colWidths[c]+2))
				} else {
					fmt.Print(cell)
				}
			}
			fmt.Println()
		}
	}
}

// printAcross prints entries left-to-right, then wrapping to next row. R1.13.
// Uses per-column widths (matching GNU ls -x) to maximize columns.
func printAcross(entries []string, termWidth int) {
	if len(entries) == 0 {
		return
	}

	// Find the maximum number of columns that fits within termWidth.
	numCols := len(entries)
	for numCols > 1 {
		colWidths := acrossColWidths(entries, numCols)
		total := 0
		for c, cw := range colWidths {
			total += cw
			if c < numCols-1 {
				total += 2
			}
		}
		if total <= termWidth {
			break
		}
		numCols--
	}

	colWidths := acrossColWidths(entries, numCols)
	for i, e := range entries {
		col := i % numCols
		if col < numCols-1 && i < len(entries)-1 {
			fmt.Print(format.PadRight(e, colWidths[col]+2))
		} else {
			fmt.Print(e)
		}
		if col == numCols-1 || i == len(entries)-1 {
			fmt.Println()
		}
	}
}

// acrossColWidths computes per-column max widths for across (left-to-right) layout.
func acrossColWidths(entries []string, numCols int) []int {
	widths := make([]int, numCols)
	for i, e := range entries {
		col := i % numCols
		w := utf8.RuneCountInString(e)
		if w > widths[col] {
			widths[col] = w
		}
	}
	return widths
}

// printLong prints entries in long format (-l).
func printLong(entries []entry, f flags) {
	maxNlink := 0
	maxSize := 0
	maxOwner := 0
	maxGroup := 0
	inodeWidth, blockWidth := metaWidths(entries, f)

	type resolvedEntry struct {
		e     entry
		owner string
		group string
		sizeS string
	}

	resolved := make([]resolvedEntry, len(entries))
	for i, e := range entries {
		var owner, group string
		if f.numericIDs {
			owner = strconv.FormatUint(uint64(e.info.Uid), 10)
			group = strconv.FormatUint(uint64(e.info.Gid), 10)
		} else {
			owner = resolveUser(e.info.Uid)
			group = resolveGroup(e.info.Gid)
		}

		var sizeS string
		if f.humanSize {
			sizeS = lsHumanSize(e.info.Size)
		} else {
			sizeS = strconv.FormatInt(e.info.Size, 10)
		}

		resolved[i] = resolvedEntry{e: e, owner: owner, group: group, sizeS: sizeS}

		nlinkS := strconv.FormatUint(e.info.Nlink, 10)
		if len(nlinkS) > maxNlink {
			maxNlink = len(nlinkS)
		}
		if len(sizeS) > maxSize {
			maxSize = len(sizeS)
		}
		if len(owner) > maxOwner {
			maxOwner = len(owner)
		}
		if len(group) > maxGroup {
			maxGroup = len(group)
		}
	}

	for _, r := range resolved {
		printMetaPrefix(r.e, f, inodeWidth, blockWidth)

		perms := permString(r.e.info.Mode)
		nlink := format.PadLeft(strconv.FormatUint(r.e.info.Nlink, 10), maxNlink)
		owner := format.PadRight(r.owner, maxOwner)
		group := format.PadRight(r.group, maxGroup)
		size := format.PadLeft(r.sizeS, maxSize)
		mtime := formatMtime(r.e.info.ModTime)

		name := colorize(displayName(r.e, f), r.e.info.Mode) + classifyChar(r.e, f)
		// R1.10: Symlink display.
		if r.e.info.Mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(r.e.path)
			if err == nil {
				name += " -> " + target
			}
		}

		fmt.Printf("%s %s %s %s %s %s %s\n", perms, nlink, owner, group, size, mtime, name)
	}
}

// permString converts an os.FileMode to a 10-character permission string. R1.6.
func permString(mode os.FileMode) string {
	var buf [10]byte

	// Position 0: file type.
	switch {
	case mode&os.ModeDir != 0:
		buf[0] = 'd'
	case mode&os.ModeSymlink != 0:
		buf[0] = 'l'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		buf[0] = 'c'
	case mode&os.ModeDevice != 0:
		buf[0] = 'b'
	case mode&os.ModeNamedPipe != 0:
		buf[0] = 'p'
	case mode&os.ModeSocket != 0:
		buf[0] = 's'
	default:
		buf[0] = '-'
	}

	perm := mode.Perm()

	// Owner.
	buf[1] = rwChar(perm, 0o400, 'r')
	buf[2] = rwChar(perm, 0o200, 'w')
	if mode&os.ModeSetuid != 0 {
		if perm&0o100 != 0 {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	} else {
		buf[3] = rwChar(perm, 0o100, 'x')
	}

	// Group.
	buf[4] = rwChar(perm, 0o040, 'r')
	buf[5] = rwChar(perm, 0o020, 'w')
	if mode&os.ModeSetgid != 0 {
		if perm&0o010 != 0 {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	} else {
		buf[6] = rwChar(perm, 0o010, 'x')
	}

	// Other.
	buf[7] = rwChar(perm, 0o004, 'r')
	buf[8] = rwChar(perm, 0o002, 'w')
	if mode&os.ModeSticky != 0 {
		if perm&0o001 != 0 {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	} else {
		buf[9] = rwChar(perm, 0o001, 'x')
	}

	return string(buf[:])
}

// rwChar returns ch if the permission bit is set, '-' otherwise.
func rwChar(perm os.FileMode, bit os.FileMode, ch byte) byte {
	if perm&bit != 0 {
		return ch
	}
	return '-'
}

// sixMonths is approximately 6 months in duration.
const sixMonths = 6 * 30 * 24 * time.Hour

// formatMtime formats a modification time matching GNU ls. R1.9.
func formatMtime(t time.Time) string {
	if time.Since(t).Abs() < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// resolveUser resolves a UID to a username, falling back to numeric ID. R1.8.
func resolveUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup resolves a GID to a group name, falling back to numeric ID. R1.8.
func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// displayName returns the display name for an entry.
func displayName(e entry, _ flags) string {
	return e.name
}

// colorize wraps a name in ANSI color codes if color is enabled. R3.3.
func colorize(name string, mode os.FileMode) string {
	c := format.FileTypeColor(mode)
	if c == "" {
		return name
	}
	return c + name + format.Reset()
}

// classifyChar returns the type indicator for -F or -p. R3.8.
func classifyChar(e entry, f flags) string {
	mode := e.info.Mode
	if f.classify {
		switch {
		case mode&os.ModeDir != 0:
			return "/"
		case mode&os.ModeSymlink != 0:
			return "@"
		case mode&os.ModeNamedPipe != 0:
			return "|"
		case mode&os.ModeSocket != 0:
			return "="
		case mode.IsRegular() && mode.Perm()&0o111 != 0:
			return "*"
		}
		return ""
	}
	if f.appendSlash && mode&os.ModeDir != 0 {
		return "/"
	}
	return ""
}

// metaWidths computes column widths for inode and block count prefixes.
func metaWidths(entries []entry, f flags) (inodeWidth, blockWidth int) {
	if f.showInode {
		for _, e := range entries {
			w := len(strconv.FormatUint(e.info.Ino, 10))
			if w > inodeWidth {
				inodeWidth = w
			}
		}
	}
	if f.showBlocks {
		for _, e := range entries {
			bk := e.info.Blocks / 2
			var w int
			if f.humanSize {
				w = len(humanSizeBlocks(bk))
			} else {
				w = len(strconv.FormatInt(bk, 10))
			}
			if w > blockWidth {
				blockWidth = w
			}
		}
	}
	return
}

// printMetaPrefix prints inode and/or block count prefix for an entry.
func printMetaPrefix(e entry, f flags, inodeWidth, blockWidth int) {
	if f.showInode {
		fmt.Print(format.PadLeft(strconv.FormatUint(e.info.Ino, 10), inodeWidth) + " ")
	}
	if f.showBlocks {
		bk := e.info.Blocks / 2
		if f.humanSize {
			fmt.Print(format.PadLeft(humanSizeBlocks(bk), blockWidth) + " ")
		} else {
			fmt.Print(format.PadLeft(strconv.FormatInt(bk, 10), blockWidth) + " ")
		}
	}
}

// lsHumanSize formats a byte count in GNU ls -lh style. R3.5.
// GNU ls always shows one decimal place for values with a suffix (e.g. "1.0K"),
// unlike pkg/format.HumanSize which drops ".0" for whole numbers.
func lsHumanSize(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	suffixes := []string{"", "K", "M", "G", "T", "P", "E"}
	f := float64(bytes)
	i := 0
	for f >= 1024 && i < len(suffixes)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d", bytes)
	}
	if f >= 10 {
		return fmt.Sprintf("%.0f%s", f, suffixes[i])
	}
	return fmt.Sprintf("%.1f%s", f, suffixes[i])
}

// humanSizeBlocks converts a 1K-block count to human-readable form. R3.7.
// Input is already in 1K-block units, so the base suffix is "K".
// GNU ls -sh shows "4.0K" for 4 1K-blocks, "0" for 0 blocks.
func humanSizeBlocks(n int64) string {
	if n == 0 {
		return "0"
	}
	suffixes := []string{"K", "M", "G", "T", "P", "E"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(suffixes)-1 {
		f /= 1024
		i++
	}
	if f >= 10 {
		return fmt.Sprintf("%.0f%s", f, suffixes[i])
	}
	return fmt.Sprintf("%.1f%s", f, suffixes[i])
}
