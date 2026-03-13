// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls lists directory contents and file arguments.
//
// Implements: prd008-ls R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7, R1.8, R1.9, R1.10, R1.11, R1.12
package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultTermWidth is used for multi-column output when the terminal width
// cannot be determined.
const defaultTermWidth = 80

// formatMode controls the output format.
type formatMode int

const (
	formatDefault formatMode = iota // auto-detect based on TTY
	formatSingle                    // -1: one entry per line
	formatLong                      // -l: long format
	formatColumns                   // -C: forced multi-column
)

// lsOptions holds parsed command-line options.
type lsOptions struct {
	format formatMode
}

func main() {
	// R1.4 / D5: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	opts := lsOptions{
		format: formatDefault,
	}

	// Parse flags. Each flag character is processed individually to support
	// combined flags like -la. The last format flag on the command line wins
	// (R1.14: -C, -x, -l, -1 override each other).
	var operands []string
	pastDash := false
	for _, arg := range args {
		if pastDash {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			pastDash = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			// Long options not yet supported.
			fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
			os.Exit(2)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			for _, ch := range arg[1:] {
				switch ch {
				case '1':
					// R1.5: single-column output.
					// GNU ls: -1 has no effect after -l (long is already one-per-line).
					if opts.format != formatLong {
						opts.format = formatSingle
					}
				case 'l':
					// R1.6: long format.
					opts.format = formatLong
				case 'C':
					// R1.11: forced multi-column output.
					opts.format = formatColumns
				default:
					fmt.Fprintf(os.Stderr, "ls: invalid option -- '%c'\n", ch)
					os.Exit(2)
				}
			}
			continue
		}
		operands = append(operands, arg)
	}

	// R1.2: default to current directory when no arguments given.
	if len(operands) == 0 {
		operands = []string{"."}
	}

	// D4: detect whether stdout is a terminal for output mode selection.
	isTTY := sys.IsTerminal(os.Stdout.Fd())

	// Separate file and directory arguments.
	var files []fileEntry
	var dirs []string
	exitCode := 0

	for _, arg := range operands {
		fi, err := sys.Lstat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", arg, unwrapMsg(err))
			// R4.2: exit 2 for cannot access a command-line argument.
			exitCode = 2
			continue
		}
		if fi.Mode.IsDir() {
			dirs = append(dirs, arg)
		} else {
			files = append(files, fileEntry{name: arg, path: arg, info: fi})
		}
	}

	// R1.2: list file arguments first, then directory arguments.
	// R1.3 / D5: sort file names in C locale byte order.
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	sort.Strings(dirs)

	needBlank := false

	// Print file arguments (not a directory listing — no "total" line).
	if len(files) > 0 {
		printFileEntries(files, isTTY, opts, false)
		needBlank = true
	}

	// Print directory arguments.
	multipleTargets := len(files) > 0 || len(dirs) > 1
	for _, dir := range dirs {
		if needBlank {
			fmt.Println()
		}
		if multipleTargets {
			fmt.Printf("%s:\n", dir)
		}
		if err := listDir(dir, isTTY, opts); err != nil {
			fmt.Fprintf(os.Stderr, "ls: reading directory '%s': %s\n", dir, unwrapMsg(err))
			if exitCode < 1 {
				exitCode = 1
			}
		}
		needBlank = true
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// fileEntry pairs an entry name with its metadata.
type fileEntry struct {
	name string
	path string // full filesystem path for operations like Readlink
	info *sys.FileInfo
}

// listDir reads and prints the contents of a single directory.
// R1.3: entries sorted lexicographically in C locale byte order.
// R1.4: dotfiles hidden by default.
func listDir(dir string, isTTY bool, opts lsOptions) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var items []fileEntry
	for _, e := range entries {
		name := e.Name()
		// R1.4 / R1.3: hide dotfiles by default.
		if strings.HasPrefix(name, ".") {
			continue
		}
		// R1.7: obtain metadata via pkg/sys.Lstat.
		path := dir + "/" + name
		fi, statErr := sys.Lstat(path)
		if statErr != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", path, unwrapMsg(statErr))
			continue
		}
		items = append(items, fileEntry{name: name, path: path, info: fi})
	}

	// R1.3 / D5: sort in C locale byte order (Go sort.Strings is byte-order).
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })

	printFileEntries(items, isTTY, opts, true)
	return nil
}

// printFileEntries outputs entries according to the current format mode.
// isDirListing controls whether the "total" block line is printed in long format.
func printFileEntries(items []fileEntry, isTTY bool, opts lsOptions, isDirListing bool) {
	if len(items) == 0 {
		if opts.format == formatLong && isDirListing {
			// GNU ls prints "total 0" for empty directories in long format.
			fmt.Println("total 0")
		}
		return
	}

	switch opts.format {
	case formatLong:
		printLong(items, isDirListing)
	case formatSingle:
		// R1.5: single-column output regardless of TTY.
		for _, item := range items {
			fmt.Println(item.name)
		}
	case formatColumns:
		// R1.11: forced multi-column regardless of TTY.
		// R1.12: vertical sort (column-major), same as format.Columns default.
		names := extractNames(items)
		termWidth := defaultTermWidth
		if isTTY {
			if tw, err := sys.TerminalWidth(); err == nil {
				termWidth = tw
			}
		}
		printColumns(names, termWidth)
	default:
		// formatDefault: auto-detect.
		names := extractNames(items)
		if !isTTY {
			// R1.2: one entry per line for non-terminal output.
			for _, name := range names {
				fmt.Println(name)
			}
			return
		}
		// R1.1 / D3: multi-column output using pkg/format.Columns.
		termWidth, err := sys.TerminalWidth()
		if err != nil {
			termWidth = defaultTermWidth
		}
		printColumns(names, termWidth)
	}
}

// printLong prints entries in long format.
// R1.6: permission string, nlink, owner, group, size, mtime, name.
// R1.7: metadata from pkg/sys.Lstat.
// R1.8: owner/group name resolution with numeric fallback.
func printLong(items []fileEntry, isDirListing bool) {
	// Compute column widths for alignment.
	maxNlink := 0
	maxOwner := 0
	maxGroup := 0
	maxSize := 0

	type resolvedEntry struct {
		perm  string
		nlink string
		owner string
		group string
		size  string
		mtime string
		name  string
	}

	resolved := make([]resolvedEntry, len(items))
	for i, item := range items {
		fi := item.info

		// R1.6: permission string.
		perm := permissionString(fi.Mode)

		// R1.7: nlink.
		nlinkStr := strconv.FormatUint(fi.Nlink, 10)

		// R1.8: resolve owner and group names.
		ownerStr := resolveUser(fi.Uid)
		groupStr := resolveGroup(fi.Gid)

		// R1.7: size.
		sizeStr := strconv.FormatInt(fi.Size, 10)

		// R1.9: modification time formatting (matching GNU ls).
		mtimeStr := formatMtime(fi.ModTime)

		// R1.10: symlink display — append " -> target" for symbolic links.
		nameStr := item.name
		if fi.Mode&os.ModeSymlink != 0 {
			if target, linkErr := os.Readlink(item.path); linkErr == nil {
				nameStr = nameStr + " -> " + target
			}
		}

		resolved[i] = resolvedEntry{
			perm:  perm,
			nlink: nlinkStr,
			owner: ownerStr,
			group: groupStr,
			size:  sizeStr,
			mtime: mtimeStr,
			name:  nameStr,
		}

		if len(nlinkStr) > maxNlink {
			maxNlink = len(nlinkStr)
		}
		if len(ownerStr) > maxOwner {
			maxOwner = len(ownerStr)
		}
		if len(groupStr) > maxGroup {
			maxGroup = len(groupStr)
		}
		if len(sizeStr) > maxSize {
			maxSize = len(sizeStr)
		}
	}

	// Print total block count line only for directory listings.
	if isDirListing {
		var totalBlocks int64
		for _, item := range items {
			totalBlocks += item.info.Blocks
		}
		// Convert 512-byte blocks to 1K-block units.
		fmt.Printf("total %d\n", totalBlocks/2)
	}

	// Print each entry.
	for _, r := range resolved {
		fmt.Printf("%s %s %s %s %s %s %s\n",
			r.perm,
			format.PadLeft(r.nlink, maxNlink),
			format.PadRight(r.owner, maxOwner),
			format.PadRight(r.group, maxGroup),
			format.PadLeft(r.size, maxSize),
			r.mtime,
			r.name,
		)
	}
}

// permissionString builds the 10-character permission string from a file mode.
// R1.6: file type indicator + owner/group/other rwx with setuid/setgid/sticky.
func permissionString(mode os.FileMode) string {
	var buf [10]byte

	// Position 0: file type.
	switch {
	case mode.IsDir():
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

	// Positions 1-3: owner permissions.
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

	// Positions 4-6: group permissions.
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

	// Positions 7-9: other permissions.
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

// rwChar returns ch if the bit is set in perm, '-' otherwise.
func rwChar(perm os.FileMode, bit os.FileMode, ch byte) byte {
	if perm&bit != 0 {
		return ch
	}
	return '-'
}

// resolveUser resolves a numeric UID to a username.
// R1.8: falls back to numeric string on lookup failure.
func resolveUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup resolves a numeric GID to a group name.
// R1.8: falls back to numeric string on lookup failure.
func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// formatMtime formats a modification time matching GNU ls behavior.
// R1.9: within 6 months uses "Jan _2 15:04", otherwise "Jan _2  2006".
func formatMtime(t time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// columnSep is the separator between columns, matching GNU ls (two spaces).
const columnSep = "  "

// printColumns renders names in multi-column layout using format.Columns.
// GNU ls uses two-space column separators, so we join with columnSep.
func printColumns(names []string, termWidth int) {
	rows := format.Columns(names, termWidth)
	for _, row := range rows {
		fmt.Println(strings.Join(row, columnSep))
	}
}

// extractNames returns just the name strings from a slice of fileEntry.
func extractNames(items []fileEntry) []string {
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.name
	}
	return names
}

// unwrapMsg extracts a user-friendly error message, stripping Go error prefixes.
// Capitalizes the first letter to match GNU coreutils error output style.
func unwrapMsg(err error) string {
	msg := err.Error()
	// Strip common prefixes like "lstat /path: " added by os/sys wrappers.
	if idx := strings.LastIndex(msg, ": "); idx >= 0 {
		msg = msg[idx+2:]
	}
	// Capitalize first letter to match GNU style ("No such file" not "no such file").
	if len(msg) > 0 && msg[0] >= 'a' && msg[0] <= 'z' {
		msg = strings.ToUpper(msg[:1]) + msg[1:]
	}
	return msg
}
