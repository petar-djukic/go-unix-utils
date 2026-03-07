// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU ls: list directory contents.
// Implements prd008-ls R1-R4 (basic listing, long format, filter flags, color, exit codes).
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

// options holds parsed command-line flags.
type options struct {
	showAll     bool // -a: include dotfiles including . and ..
	almostAll   bool // -A: include dotfiles excluding . and ..
	onePerLine  bool // -1: force single-column
	longFormat  bool // -l: long format
	recursive   bool // -R: recursive listing
	dirOnly     bool // -d: list directory entries themselves
	humanSize   bool // -h: human-readable sizes with -l
	colorMode   string // "always", "auto", "never"
}

func main() {
	// D1: SIGPIPE handler per ARCHITECTURE.yaml.
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	// R4.3: Set color mode.
	switch opts.colorMode {
	case "always":
		format.SetColorEnabled(true)
	case "never":
		format.SetColorEnabled(false)
	case "auto":
		format.SetColorEnabled(sys.IsTerminal(os.Stdout.Fd()))
	}

	// Determine output format. R1.2: non-TTY defaults to single-column.
	isTTY := sys.IsTerminal(os.Stdout.Fd())
	singleColumn := !isTTY || opts.onePerLine

	termWidth := 80
	if isTTY {
		if w, err := sys.TerminalWidth(); err == nil && w > 0 {
			termWidth = w
		}
	}

	if len(files) == 0 {
		files = []string{"."}
	}

	exitCode := 0

	// R2: Separate file args from directory args.
	var fileArgs []string
	var dirArgs []string

	for _, f := range files {
		fi, err := sys.Lstat(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': No such file or directory\n", f)
			exitCode = 1
			continue
		}
		if fi.Mode.IsDir() && !opts.dirOnly {
			dirArgs = append(dirArgs, f)
		} else {
			fileArgs = append(fileArgs, f)
		}
	}

	sort.Strings(fileArgs)
	sort.Strings(dirArgs)

	needBlank := false

	// R2: List file arguments first.
	if len(fileArgs) > 0 {
		if opts.longFormat {
			printLongFiles(fileArgs, opts)
		} else if singleColumn {
			for _, f := range fileArgs {
				printColorized(filepath.Base(f), f)
				fmt.Println()
			}
		} else {
			names := make([]string, len(fileArgs))
			for i, f := range fileArgs {
				names[i] = filepath.Base(f)
			}
			printColumns(names, fileArgs, termWidth)
		}
		needBlank = true
	}

	// R2: List directory arguments.
	showHeader := len(dirArgs) > 1 || len(fileArgs) > 0
	for i, dir := range dirArgs {
		if needBlank {
			fmt.Println()
		}
		if showHeader {
			fmt.Printf("%s:\n", dir)
		}
		if err := listDir(dir, opts, singleColumn, termWidth); err != nil {
			exitCode = 1
		}
		if i < len(dirArgs)-1 {
			needBlank = true
		}
	}

	// R3: -R recursive listing.
	if opts.recursive && len(dirArgs) > 0 {
		for _, dir := range dirArgs {
			if err := listRecursive(dir, opts, singleColumn, termWidth); err != nil {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

// listDir lists the contents of a single directory.
func listDir(dir string, opts options, singleColumn bool, termWidth int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': %s\n", dir, err.Error())
		return err
	}

	// R1.4, R3.1, R3.2: Filter entries.
	var names []string
	if opts.showAll {
		names = append(names, ".", "..")
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !opts.showAll && !opts.almostAll {
			continue
		}
		names = append(names, name)
	}

	// R1.3: Sort in C locale order.
	sort.Strings(names)

	if opts.longFormat {
		printLongDir(dir, names, opts)
	} else if singleColumn {
		for _, name := range names {
			path := filepath.Join(dir, name)
			printColorized(name, path)
			fmt.Println()
		}
	} else {
		paths := make([]string, len(names))
		for i, name := range names {
			paths[i] = filepath.Join(dir, name)
		}
		printColumns(names, paths, termWidth)
	}
	return nil
}

// listRecursive recursively lists subdirectories (R3 -R).
func listRecursive(dir string, opts options, singleColumn bool, termWidth int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var subdirs []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !opts.showAll && !opts.almostAll {
			continue
		}
		if e.IsDir() && name != "." && name != ".." {
			subdirs = append(subdirs, name)
		}
	}
	sort.Strings(subdirs)

	var retErr error
	for _, sub := range subdirs {
		subPath := filepath.Join(dir, sub)
		fmt.Printf("\n%s:\n", subPath)
		if err := listDir(subPath, opts, singleColumn, termWidth); err != nil {
			retErr = err
		}
		if err := listRecursive(subPath, opts, singleColumn, termWidth); err != nil {
			retErr = err
		}
	}
	return retErr
}

// printColorized prints a name with ANSI color codes if color is enabled.
func printColorized(name, path string) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Print(name)
		return
	}
	color := format.FileTypeColor(fi.Mode)
	reset := format.Reset()
	fmt.Printf("%s%s%s", color, name, reset)
}

// printColumns prints names in multi-column format.
func printColumns(names []string, paths []string, termWidth int) {
	// Build display names with color codes.
	displayNames := make([]string, len(names))
	for i, name := range names {
		fi, err := sys.Lstat(paths[i])
		if err != nil {
			displayNames[i] = name
			continue
		}
		color := format.FileTypeColor(fi.Mode)
		reset := format.Reset()
		displayNames[i] = color + name + reset
	}

	rows := format.Columns(names, termWidth)
	for _, row := range rows {
		for j, entry := range row {
			// Find index in names to get the display name.
			idx := -1
			for k, n := range names {
				if n == entry {
					idx = k
					break
				}
			}
			display := entry
			if idx >= 0 {
				display = displayNames[idx]
			}
			if j < len(row)-1 {
				// Pad to column width using the plain name length.
				padWidth := colWidthForEntry(entry, rows) - utf8.RuneCountInString(entry)
				fmt.Printf("%s%s", display, strings.Repeat(" ", padWidth+2))
			} else {
				fmt.Print(display)
			}
		}
		fmt.Println()
	}
}

// colWidthForEntry finds the column width for an entry based on the column it's in.
func colWidthForEntry(entry string, rows [][]string) int {
	// Find which column this entry is in and compute that column's max width.
	col := -1
	for _, row := range rows {
		for j, e := range row {
			if e == entry {
				col = j
				break
			}
		}
		if col >= 0 {
			break
		}
	}
	if col < 0 {
		return utf8.RuneCountInString(entry)
	}
	maxW := 0
	for _, row := range rows {
		if col < len(row) {
			w := utf8.RuneCountInString(row[col])
			if w > maxW {
				maxW = w
			}
		}
	}
	return maxW
}

// entryInfo holds metadata for a single listed entry.
type entryInfo struct {
	name string
	fi   *sys.FileInfo
	path string
}

// printLongFiles prints file arguments in long format (no total line).
func printLongFiles(files []string, opts options) {
	var infos []entryInfo
	for _, f := range files {
		fi, err := sys.Lstat(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", f, err.Error())
			continue
		}
		infos = append(infos, entryInfo{name: filepath.Base(f), fi: fi, path: f})
	}
	if len(infos) == 0 {
		return
	}

	nlinkW, ownerW, groupW, sizeW := computeFieldWidths(infos)
	for _, e := range infos {
		printLongEntry(e.name, e.fi, e.path, opts, nlinkW, ownerW, groupW, sizeW)
	}
}

// printLongDir prints directory contents in long format with total line.
func printLongDir(dir string, names []string, opts options) {
	var infos []entryInfo
	var totalBlocks int64

	for _, name := range names {
		path := filepath.Join(dir, name)
		fi, err := sys.Lstat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", path, err.Error())
			continue
		}
		infos = append(infos, entryInfo{name: name, fi: fi, path: path})
		totalBlocks += fi.Blocks
	}

	// R2.6: total line (512-byte blocks to 1K-blocks).
	totalK := totalBlocks / 2
	if opts.humanSize {
		fmt.Printf("total %s\n", format.HumanSize(totalK*1024, format.HumanSizeOpts{Binary: true}))
	} else {
		fmt.Printf("total %d\n", totalK)
	}

	if len(infos) == 0 {
		return
	}

	nlinkW, ownerW, groupW, sizeW := computeFieldWidths(infos)
	for _, e := range infos {
		printLongEntry(e.name, e.fi, e.path, opts, nlinkW, ownerW, groupW, sizeW)
	}
}

// computeFieldWidths calculates column widths for long format fields.
func computeFieldWidths(infos []entryInfo) (nlinkW, ownerW, groupW, sizeW int) {
	for _, e := range infos {
		nw := len(strconv.FormatUint(e.fi.Nlink, 10))
		if nw > nlinkW {
			nlinkW = nw
		}
		ow := len(lookupUser(e.fi.Uid))
		if ow > ownerW {
			ownerW = ow
		}
		gw := len(lookupGroup(e.fi.Gid))
		if gw > groupW {
			groupW = gw
		}
		sw := len(strconv.FormatInt(e.fi.Size, 10))
		if sw > sizeW {
			sizeW = sw
		}
	}
	return
}

// printLongEntry prints a single long-format line.
func printLongEntry(name string, fi *sys.FileInfo, path string, opts options, nlinkW, ownerW, groupW, sizeW int) {
	perms := permString(fi.Mode)
	nlink := strconv.FormatUint(fi.Nlink, 10)
	owner := lookupUser(fi.Uid)
	group := lookupGroup(fi.Gid)

	var sizeStr string
	if opts.humanSize {
		sizeStr = format.HumanSize(fi.Size, format.HumanSizeOpts{Binary: true})
		if sizeW < len(sizeStr) {
			sizeW = len(sizeStr)
		}
	} else {
		sizeStr = strconv.FormatInt(fi.Size, 10)
	}

	timeStr := formatModTime(fi.ModTime)

	displayName := name
	// R2.6: Symlink display.
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err == nil {
			displayName = name + " -> " + target
		}
	}

	// Apply color to name portion only.
	color := format.FileTypeColor(fi.Mode)
	reset := format.Reset()
	if color != "" {
		if fi.Mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err == nil {
				displayName = color + name + reset + " -> " + target
			} else {
				displayName = color + name + reset
			}
		} else {
			displayName = color + name + reset
		}
	}

	fmt.Printf("%s %*s %-*s %-*s %*s %s %s\n",
		perms,
		nlinkW, nlink,
		ownerW, owner,
		groupW, group,
		sizeW, sizeStr,
		timeStr,
		displayName,
	)
}

// permString builds the 10-character permission string. R2.2.
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

	// Owner permissions.
	perm := mode.Perm()
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

	// Group permissions.
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

	// Other permissions.
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

// rwChar returns ch if bit is set in perm, '-' otherwise.
func rwChar(perm os.FileMode, bit os.FileMode, ch byte) byte {
	if perm&bit != 0 {
		return ch
	}
	return '-'
}

// formatModTime formats modification time per R2.5.
func formatModTime(t time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	if time.Since(t) < sixMonths && time.Since(t) > -sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// lookupUser resolves a UID to a username. R2.4.
func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// lookupGroup resolves a GID to a group name. R2.4.
func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// parseArgs parses ls command-line flags. R4, R6.3.
func parseArgs(args []string) (options, []string) {
	opts := options{colorMode: "auto"}
	var files []string
	endFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endFlags {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		// R4: --help and --version.
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("ls (go-unix-utils) dev")
			os.Exit(0)
		}

		// Long options.
		if strings.HasPrefix(arg, "--color") {
			if arg == "--color" {
				opts.colorMode = "always"
			} else if strings.HasPrefix(arg, "--color=") {
				val := arg[len("--color="):]
				switch val {
				case "always", "yes", "force":
					opts.colorMode = "always"
				case "never", "no", "none":
					opts.colorMode = "never"
				case "auto", "tty", "if-tty":
					opts.colorMode = "auto"
				default:
					fmt.Fprintf(os.Stderr, "ls: invalid argument '%s' for '--color'\n", val)
					os.Exit(2)
				}
			} else {
				fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
				os.Exit(2)
			}
			continue
		}

		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
			os.Exit(2)
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					opts.showAll = true
					opts.almostAll = false
				case 'A':
					opts.almostAll = true
					opts.showAll = false
				case '1':
					opts.onePerLine = true
				case 'l':
					opts.longFormat = true
				case 'R':
					opts.recursive = true
				case 'd':
					opts.dirOnly = true
				case 'h':
					opts.humanSize = true
				case 'q':
					// force ? for non-printable chars (not implemented, accept silently)
				default:
					fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
					os.Exit(2)
				}
			}
			continue
		}

		if arg == "-" {
			files = append(files, arg)
			continue
		}

		files = append(files, arg)
	}

	return opts, files
}

// printHelp prints usage to stdout. R4.
func printHelp() {
	fmt.Println("Usage: ls [OPTION]... [FILE]...")
	fmt.Println("List information about the FILEs (the current directory by default).")
	fmt.Println("Sort entries alphabetically if none of -cftuvSUX nor --sort is specified.")
	fmt.Println()
	fmt.Println("Mandatory arguments to long options are mandatory for short options too.")
	fmt.Println("  -a, --all                  do not ignore entries starting with .")
	fmt.Println("  -A, --almost-all           do not list implied . and ..")
	fmt.Println("  -d, --directory            list directories themselves, not their contents")
	fmt.Println("  -h, --human-readable       with -l, print sizes like 1K 234M 2G etc.")
	fmt.Println("  -l                         use a long listing format")
	fmt.Println("  -R, --recursive            list subdirectories recursively")
	fmt.Println("  -1                         list one file per line")
	fmt.Println("      --color[=WHEN]         colorize the output; WHEN can be 'always',")
	fmt.Println("                               'auto', or 'never'; more info below")
	fmt.Println("      --help        display this help and exit")
	fmt.Println("      --version     output version information and exit")
}
