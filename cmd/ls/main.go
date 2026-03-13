// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls lists directory contents and file arguments.
//
// Implements: prd008-ls R1.1-R1.14, R2.1-R2.15, R3.1-R3.15, R4.1-R4.4
package main

import (
	"fmt"
	"math"
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
	formatDefault    formatMode = iota // auto-detect based on TTY
	formatSingle                       // -1: one entry per line
	formatLong                         // -l: long format
	formatColumns                      // -C: forced multi-column (vertical)
	formatHorizontal                   // -x: forced multi-column (horizontal)
)

// filterMode controls which entries are shown.
type filterMode int

const (
	filterDefault   filterMode = iota // hide dotfiles
	filterAll                         // -a: show all including . and ..
	filterAlmostAll                   // -A: show dotfiles except . and ..
)

// colorMode controls colorized output.
type colorMode int

const (
	colorNever  colorMode = iota // --color=never: no ANSI escape sequences
	colorAuto                    // --color=auto: colorize only when stdout is a TTY
	colorAlways                  // --color=always: always emit ANSI escape sequences
)

// sortMode controls how entries are ordered.
type sortMode int

const (
	sortName     sortMode = iota // default: lexicographic C locale
	sortTime                     // -t: newest first by mtime
	sortSize                     // -S: largest first by size
	sortUnsorted                 // -U: directory order (no sorting)
	sortVersion                  // -v: natural version sort
)

// lsOptions holds parsed command-line options.
type lsOptions struct {
	format     formatMode
	filter     filterMode
	sortBy     sortMode
	reverse    bool // -r: reverse sort order
	dirOnly    bool // -d: list directories themselves, not contents
	showInode  bool // -i: prepend inode number
	showBlocks bool // -s: prepend allocated block count
	numericIDs  bool      // -n: numeric UID/GID (implies -l)
	humanSizes  bool      // -h: human-readable sizes (only with -l)
	classify    bool      // -F: append type indicator character
	recursive   bool      // -R: recursively list subdirectories
	color       colorMode // --color: colorization mode
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
			// R3.1: --color=always|auto|never; --color without value defaults to "always".
			if arg == "--color" {
				opts.color = colorAlways
				continue
			}
			if strings.HasPrefix(arg, "--color=") {
				val := arg[len("--color="):]
				switch val {
				case "always":
					opts.color = colorAlways
				case "auto":
					opts.color = colorAuto
				case "never":
					opts.color = colorNever
				default:
					fmt.Fprintf(os.Stderr, "ls: invalid argument '%s' for '--color'\n", val)
					os.Exit(2)
				}
				continue
			}
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
					// R1.11: forced multi-column output (vertical).
					opts.format = formatColumns
				case 'x':
					// R1.13: forced multi-column output (horizontal).
					opts.format = formatHorizontal
				case 'a':
					// R2.1: show all entries including . and ..
					opts.filter = filterAll
				case 'A':
					// R2.2: show dotfiles except . and ..
					opts.filter = filterAlmostAll
				case 'd':
					// R2.3: list directories themselves, not contents.
					opts.dirOnly = true
				case 't':
					// R2.5: sort by modification time, newest first.
					opts.sortBy = sortTime
				case 'S':
					// R2.6: sort by file size, largest first.
					opts.sortBy = sortSize
				case 'r':
					// R2.7: reverse the current sort order.
					opts.reverse = true
				case 'U':
					// R2.8: disable sorting, list in directory order.
					opts.sortBy = sortUnsorted
				case 'v':
					// R2.9: natural version sort.
					opts.sortBy = sortVersion
			case 'i':
				// R2.11: prepend inode number.
				opts.showInode = true
			case 's':
				// R2.12: prepend allocated block count.
				opts.showBlocks = true
			case 'h':
				// R3.5: human-readable sizes (only effective with -l).
				opts.humanSizes = true
			case 'F':
				// R3.8: append type indicator character after each entry name.
				opts.classify = true
			case 'R':
				// R3.11: recursively list subdirectory contents.
				opts.recursive = true
			case 'n':
				// R2.14 / R4.6: numeric UID/GID, implies -l.
				opts.numericIDs = true
				opts.format = formatLong
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

	// R3.1-R3.3: configure color output based on --color flag.
	switch opts.color {
	case colorAlways:
		format.SetColorEnabled(true)
	case colorNever:
		format.SetColorEnabled(false)
	case colorAuto:
		// R3.2: colorize only when stdout is a TTY.
		format.SetColorEnabled(isTTY)
	}

	// Separate file and directory arguments.
	var files []fileEntry
	var dirs []string
	exitCode := 0
	failedArgs := 0

	for _, arg := range operands {
		fi, err := sys.Lstat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", arg, unwrapMsg(err))
			// R4.2: exit 2 for cannot access a command-line argument.
			exitCode = 2
			failedArgs++
			continue
		}
		// R2.3: with -d, treat directories as file entries (don't descend).
		if fi.Mode.IsDir() && !opts.dirOnly {
			dirs = append(dirs, arg)
		} else {
			files = append(files, fileEntry{name: arg, path: arg, info: fi})
		}
	}

	// R1.2: list file arguments first, then directory arguments.
	// R1.3 / D5: sort file names in C locale byte order.
	sortEntries(files, opts.sortBy)
	// R2.7: reverse sort order if -r is given.
	// R2.8: -r with -U has no effect (directory order is not reversible).
	if opts.reverse && opts.sortBy != sortUnsorted {
		reverseEntries(files)
	}
	sort.Strings(dirs)

	needBlank := false

	// Print file arguments (not a directory listing — no "total" line).
	if len(files) > 0 {
		printFileEntries(files, isTTY, opts, false)
		needBlank = true
	}

	// Print directory arguments.
	// R3.11: when -R is active, always print headers (each subdir gets one).
	// R4.2: failed args count toward multiple targets for directory header display.
	multipleTargets := len(files) > 0 || len(dirs) > 1 || failedArgs > 0 || opts.recursive
	for _, dir := range dirs {
		if needBlank {
			fmt.Println()
		}
		if multipleTargets {
			fmt.Printf("%s:\n", dir)
		}
		if err := listDir(dir, isTTY, opts, &exitCode); err != nil {
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

// sortEntries sorts a slice of fileEntry according to the given sort mode.
// R2.5: -t sorts by modification time, newest first, with name tiebreaker.
// R2.6: -S sorts by file size, largest first, with name tiebreaker.
// R2.7: reverse inverts the final order.
// R2.8: sortUnsorted skips sorting entirely.
// R2.9: sortVersion uses natural version sort (strverscmp).
func sortEntries(items []fileEntry, mode sortMode) {
	switch mode {
	case sortUnsorted:
		// R2.8: no sorting, preserve directory order.
		return
	case sortTime:
		sort.SliceStable(items, func(i, j int) bool {
			ti := items[i].info.ModTime
			tj := items[j].info.ModTime
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return items[i].name < items[j].name
		})
	case sortSize:
		sort.SliceStable(items, func(i, j int) bool {
			si := items[i].info.Size
			sj := items[j].info.Size
			if si != sj {
				return si > sj
			}
			return items[i].name < items[j].name
		})
	case sortVersion:
		// R2.9: natural version sort using strverscmp semantics.
		sort.SliceStable(items, func(i, j int) bool {
			return strverscmp(items[i].name, items[j].name) < 0
		})
	default:
		sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
	}
}

// reverseEntries reverses a slice of fileEntry in place.
// R2.7: -r reverses the current sort order.
func reverseEntries(items []fileEntry) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

// strverscmp compares two strings using natural version sort semantics,
// matching the glibc strverscmp behavior used by GNU ls -v.
// R2.9: runs of digits are compared numerically, not lexicographically.
func strverscmp(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		// Both digits: compare numerically.
		if isDigit(ca) && isDigit(cb) {
			// Skip leading zeros and compare digit runs.
			cmp := compareDigitRuns(a[i:], b[j:])
			if cmp != 0 {
				return cmp
			}
			// Advance past the digit runs.
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			continue
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		i++
		j++
	}
	return len(a) - len(b)
}

// compareDigitRuns compares two strings that start with digit runs numerically.
// Handles leading zeros correctly: shorter numeric value wins; on equal value,
// fewer leading zeros wins (matching strverscmp).
func compareDigitRuns(a, b string) int {
	// Count leading zeros.
	az, bz := 0, 0
	for az < len(a) && a[az] == '0' {
		az++
	}
	for bz < len(b) && b[bz] == '0' {
		bz++
	}

	// Find lengths of digit runs after leading zeros.
	ai := az
	for ai < len(a) && isDigit(a[ai]) {
		ai++
	}
	bi := bz
	for bi < len(b) && isDigit(b[bi]) {
		bi++
	}

	aLen := ai - az // significant digits in a
	bLen := bi - bz // significant digits in b

	// Longer significant digit run is larger.
	if aLen != bLen {
		return aLen - bLen
	}

	// Same length: compare digit by digit.
	for k := range aLen {
		if a[az+k] != b[bz+k] {
			return int(a[az+k]) - int(b[bz+k])
		}
	}

	// Equal numeric value: more leading zeros sorts first.
	if az != bz {
		return bz - az
	}

	return 0
}

// isDigit returns true if b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// listDir reads and prints the contents of a single directory.
// R1.3: entries sorted lexicographically in C locale byte order.
// R1.4: dotfiles hidden by default.
// R3.11-R3.15: when -R is active, recurse into subdirectories.
func listDir(dir string, isTTY bool, opts lsOptions, exitCode *int) error {
	// R2.8: for -U (unsorted), use raw readdir to preserve filesystem order.
	// os.ReadDir sorts entries alphabetically, which defeats -U.
	names, err := readDirNames(dir, opts.sortBy == sortUnsorted)
	if err != nil {
		return err
	}

	var items []fileEntry

	// R2.1: -a adds "." and ".." entries.
	if opts.filter == filterAll {
		for _, dotEntry := range []string{".", ".."} {
			dotPath := dir + "/" + dotEntry
			fi, statErr := sys.Lstat(dotPath)
			if statErr != nil {
				fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", dotPath, unwrapMsg(statErr))
				continue
			}
			items = append(items, fileEntry{name: dotEntry, path: dotPath, info: fi})
		}
	}

	for _, name := range names {
		// R1.4: hide dotfiles by default.
		// R2.1/R2.2: -a and -A include dotfiles.
		if strings.HasPrefix(name, ".") && opts.filter == filterDefault {
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

	// Sort entries according to the active sort mode.
	sortEntries(items, opts.sortBy)
	// R2.7: reverse sort order if -r is given.
	// R2.8: -r with -U has no effect (directory order is not reversible).
	if opts.reverse && opts.sortBy != sortUnsorted {
		reverseEntries(items)
	}

	printFileEntries(items, isTTY, opts, true)

	// R3.11: recursively list subdirectories when -R is active.
	// R3.13: do not follow symbolic links to directories.
	// R3.14: filter flags (-a, -A) apply to each subdirectory.
	// R3.15: subdirectories are listed in the same sort order as entries.
	if opts.recursive {
		for _, item := range items {
			// Skip "." and ".." to avoid infinite recursion.
			if item.name == "." || item.name == ".." {
				continue
			}
			// R3.13: only recurse into real directories, not symlinks.
			if !item.info.Mode.IsDir() {
				continue
			}
			fmt.Println()
			fmt.Printf("%s:\n", item.path)
			if recurseErr := listDir(item.path, isTTY, opts, exitCode); recurseErr != nil {
				fmt.Fprintf(os.Stderr, "ls: reading directory '%s': %s\n", item.path, unwrapMsg(recurseErr))
				if *exitCode < 1 {
					*exitCode = 1
				}
			}
		}
	}

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

	// R2.12/R2.13/R3.7: print total block line for -s directory listings (all formats).
	if opts.showBlocks && isDirListing && opts.format != formatLong {
		var totalBlocks int64
		for _, item := range items {
			totalBlocks += item.info.Blocks
		}
		if opts.humanSizes {
			// R3.7: convert 512-byte blocks to bytes for human-readable display.
			fmt.Printf("total %s\n", humanSizeGNU(totalBlocks*512))
		} else {
			fmt.Printf("total %d\n", totalBlocks/2)
		}
	}

	switch opts.format {
	case formatLong:
		printLong(items, isDirListing, opts)
	case formatSingle:
		// R1.5: single-column output regardless of TTY.
		// R2.11/R2.12/R2.15: prepend inode and/or blocks if requested.
		names := extractPrefixedNames(items, opts)
		for _, name := range names {
			fmt.Println(name)
		}
	case formatColumns:
		// R1.11: forced multi-column regardless of TTY.
		// R1.12: vertical sort (column-major), same as format.Columns default.
		names := extractPrefixedNames(items, opts)
		termWidth := defaultTermWidth
		if isTTY {
			if tw, err := sys.TerminalWidth(); err == nil {
				termWidth = tw
			}
		}
		printColumns(names, termWidth)
	case formatHorizontal:
		// R1.13: forced multi-column with horizontal (row-major) fill.
		names := extractPrefixedNames(items, opts)
		termWidth := defaultTermWidth
		if isTTY {
			if tw, err := sys.TerminalWidth(); err == nil {
				termWidth = tw
			}
		}
		printHorizontalColumns(names, termWidth)
	default:
		// formatDefault: auto-detect.
		names := extractPrefixedNames(items, opts)
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
// R2.11: -i prepends inode number.
// R2.12: -s prepends block count.
// R2.14: -n uses numeric UID/GID.
func printLong(items []fileEntry, isDirListing bool, opts lsOptions) {
	// Compute column widths for alignment.
	maxIno := 0
	maxBlk := 0
	maxNlink := 0
	maxOwner := 0
	maxGroup := 0
	maxSize := 0

	type resolvedEntry struct {
		ino   string
		blk   string
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

		// R2.11: inode number.
		var inoStr string
		if opts.showInode {
			inoStr = strconv.FormatUint(fi.Ino, 10)
			if len(inoStr) > maxIno {
				maxIno = len(inoStr)
			}
		}

		// R2.12 / R3.7: allocated block count in 1K units; human-readable when -h.
		var blkStr string
		if opts.showBlocks {
			if opts.humanSizes {
				// R3.7: convert 512-byte blocks to bytes for human-readable display.
				blkStr = humanSizeGNU(fi.Blocks * 512)
			} else {
				blkStr = strconv.FormatInt(fi.Blocks/2, 10)
			}
			if len(blkStr) > maxBlk {
				maxBlk = len(blkStr)
			}
		}

		// R1.6: permission string.
		perm := permissionString(fi.Mode)

		// R1.7: nlink.
		nlinkStr := strconv.FormatUint(fi.Nlink, 10)

		// R2.14: numeric UID/GID; R1.8: resolve owner and group names.
		var ownerStr, groupStr string
		if opts.numericIDs {
			ownerStr = strconv.FormatUint(uint64(fi.Uid), 10)
			groupStr = strconv.FormatUint(uint64(fi.Gid), 10)
		} else {
			ownerStr = resolveUser(fi.Uid)
			groupStr = resolveGroup(fi.Gid)
		}

		// R1.7 / R3.5: size — human-readable when -h is active with -l.
		var sizeStr string
		if opts.humanSizes {
			sizeStr = humanSizeGNU(fi.Size)
		} else {
			sizeStr = strconv.FormatInt(fi.Size, 10)
		}

		// R1.9: modification time formatting (matching GNU ls).
		mtimeStr := formatMtime(fi.ModTime)

		// R3.3: colorize entry name based on file type.
		// R3.8/R3.10: -F appends type indicator after ANSI reset.
		// In long format, symlinks show " -> target" which already indicates the type,
		// so GNU ls omits the "@" indicator for symlinks in long format.
		classifyInLong := opts.classify
		isSymlink := fi.Mode&os.ModeSymlink != 0
		if isSymlink && classifyInLong {
			classifyInLong = false
		}
		nameStr := colorizeEntry(item.name, fi.Mode, classifyInLong)
		// R1.10: symlink display — append " -> target" for symbolic links.
		if isSymlink {
			if target, linkErr := os.Readlink(item.path); linkErr == nil {
				nameStr = nameStr + " -> " + target
			}
		}

		resolved[i] = resolvedEntry{
			ino:   inoStr,
			blk:   blkStr,
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
	// R3.6: when -h is active, convert total to human-readable form.
	if isDirListing {
		var totalBlocks int64
		for _, item := range items {
			totalBlocks += item.info.Blocks
		}
		if opts.humanSizes {
			// R3.6: convert 512-byte blocks to bytes for human-readable display.
			fmt.Printf("total %s\n", humanSizeGNU(totalBlocks*512))
		} else {
			// Convert 512-byte blocks to 1K-block units.
			fmt.Printf("total %d\n", totalBlocks/2)
		}
	}

	// Print each entry.
	for _, r := range resolved {
		// R2.15: inode first, then block count, then long-format fields.
		var prefix string
		if opts.showInode {
			prefix += format.PadLeft(r.ino, maxIno) + " "
		}
		if opts.showBlocks {
			prefix += format.PadLeft(r.blk, maxBlk) + " "
		}
		fmt.Printf("%s%s %s %s %s %s %s %s\n",
			prefix,
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

// tabStop is the default tab stop width used by GNU ls for column alignment.
const tabStop = 8

// printHorizontalColumns renders names in horizontal multi-column layout
// with tab-based padding matching GNU ls behavior.
// R1.13: entries fill across columns first, then down to the next row.
func printHorizontalColumns(names []string, termWidth int) {
	n := len(names)
	if n == 0 {
		return
	}
	if termWidth <= 0 {
		for _, name := range names {
			fmt.Println(name)
		}
		return
	}

	// Find max columns that fit. Column width = max_entry_width + 2 (matching GNU ls).
	ncols := 1
	var colWidths []int
	for tryNCols := n; tryNCols >= 1; tryNCols-- {
		// Row-major fill: entry i is in column i % tryNCols.
		cw := make([]int, tryNCols)
		for i, e := range names {
			col := i % tryNCols
			if w := len(e); w > cw[col] {
				cw[col] = w
			}
		}

		// Total = sum of (colWidth + 2) for non-last columns + last column width.
		total := 0
		for c, w := range cw {
			total += w
			if c < tryNCols-1 {
				total += 2
			}
		}

		if total <= termWidth {
			ncols = tryNCols
			colWidths = cw
			break
		}
	}

	nrows := (n + ncols - 1) / ncols

	// Compute column start positions.
	colStarts := make([]int, ncols)
	for c := 1; c < ncols; c++ {
		colStarts[c] = colStarts[c-1] + colWidths[c-1] + 2
	}

	// Print entries row by row with tab-based padding (matching GNU ls).
	for row := range nrows {
		var line strings.Builder
		pos := 0
		for col := range ncols {
			idx := row*ncols + col
			if idx >= n {
				break
			}

			// Advance to column start using tabs and spaces.
			if col > 0 {
				target := colStarts[col]
				pos = writeTabPad(&line, pos, target)
			}

			entry := names[idx]
			line.WriteString(entry)
			pos += len(entry)
		}
		fmt.Println(line.String())
	}
}

// writeTabPad writes a mix of tab and space characters to advance from
// currentPos to targetPos, matching GNU ls column padding behavior.
// Returns the new position after padding.
func writeTabPad(buf *strings.Builder, currentPos, targetPos int) int {
	pos := currentPos
	for pos < targetPos {
		nextTab := ((pos / tabStop) + 1) * tabStop
		if nextTab <= targetPos {
			buf.WriteByte('\t')
			pos = nextTab
		} else {
			buf.WriteByte(' ')
			pos++
		}
	}
	return pos
}

// readDirNames returns the names of directory entries. When raw is true,
// entries are returned in filesystem order (for -U); otherwise sorted
// alphabetically (matching os.ReadDir behavior).
func readDirNames(dir string, raw bool) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	dirNames, err := f.Readdirnames(-1)
	f.Close() // best-effort close; read already succeeded or failed
	if err != nil {
		return nil, err
	}
	if !raw {
		sort.Strings(dirNames)
	}
	return dirNames, nil
}

// extractPrefixedNames returns entry names with optional inode/block prefixes and color.
// R2.11: -i prepends inode number. R2.12: -s prepends block count.
// R2.15: when both are given, inode is printed first, then block count.
// R3.3: entry names are colorized based on file type when color is enabled.
func extractPrefixedNames(items []fileEntry, opts lsOptions) []string {
	// Compute max widths for right-alignment.
	maxInoWidth := 0
	maxBlkWidth := 0
	if opts.showInode {
		for _, item := range items {
			w := len(strconv.FormatUint(item.info.Ino, 10))
			if w > maxInoWidth {
				maxInoWidth = w
			}
		}
	}
	if opts.showBlocks {
		for _, item := range items {
			var blk string
			if opts.humanSizes {
				// R3.7: convert 512-byte blocks to bytes for human-readable display.
				blk = humanSizeGNU(item.info.Blocks * 512)
			} else {
				blk = strconv.FormatInt(item.info.Blocks/2, 10)
			}
			if w := len(blk); w > maxBlkWidth {
				maxBlkWidth = w
			}
		}
	}

	names := make([]string, len(items))
	for i, item := range items {
		var prefix string
		if opts.showInode {
			inoStr := strconv.FormatUint(item.info.Ino, 10)
			prefix += format.PadLeft(inoStr, maxInoWidth) + " "
		}
		if opts.showBlocks {
			var blkStr string
			if opts.humanSizes {
				// R3.7: convert 512-byte blocks to bytes for human-readable display.
				blkStr = humanSizeGNU(item.info.Blocks * 512)
			} else {
				blkStr = strconv.FormatInt(item.info.Blocks/2, 10)
			}
			prefix += format.PadLeft(blkStr, maxBlkWidth) + " "
		}
		// R3.3: colorize entry name based on file type.
		// R3.8/R3.10: -F appends type indicator.
		names[i] = prefix + colorizeEntry(item.name, item.info.Mode, opts.classify)
	}
	return names
}

// colorFirstDone tracks whether the initial ANSI reset has been emitted.
// GNU ls emits a reset sequence (\033[0m) before the very first colored entry
// name in the output to clear any pre-existing terminal color state. Subsequent
// colored entries rely on the trailing reset from the previous entry.
var colorFirstDone bool

// colorizeEntry wraps a name with ANSI color codes based on file type.
// R3.3: uses pkg/format.FileTypeColor for the opening sequence and
// pkg/format.Reset for the closing sequence. When color is disabled,
// FileTypeColor returns "" and no escape sequences appear in output.
// R3.8/R3.10: when classify is true, appends the type indicator after the
// ANSI reset sequence so the indicator is not colorized.
func colorizeEntry(name string, mode os.FileMode, classify bool) string {
	indicator := ""
	if classify {
		indicator = typeIndicator(mode)
	}
	colorCode := format.FileTypeColor(mode)
	if colorCode == "" {
		return name + indicator
	}
	// GNU ls emits a reset before the first colored entry only.
	var prefix string
	if !colorFirstDone {
		prefix = format.Reset()
		colorFirstDone = true
	}
	return prefix + colorCode + name + format.Reset() + indicator
}

// typeIndicator returns the -F classification suffix for the given file mode.
// R3.8: "/" for directories, "*" for executable regular files, "@" for symlinks,
// "|" for named pipes, "=" for sockets. Regular non-executable files get "".
// R3.9: executable is defined as any execute bit set (fi.Mode&0o111 != 0).
func typeIndicator(mode os.FileMode) string {
	switch {
	case mode.IsDir():
		return "/"
	case mode&os.ModeSymlink != 0:
		return "@"
	case mode&os.ModeNamedPipe != 0:
		return "|"
	case mode&os.ModeSocket != 0:
		return "="
	case mode.IsRegular() && mode.Perm()&0o111 != 0:
		return "*"
	default:
		return ""
	}
}

// humanSizeGNU formats a byte count in human-readable binary (1024-base) form
// with ceiling rounding, matching GNU coreutils human_readable() behavior.
// R3.5/R3.6/R3.7: used for -h file sizes, total line, and block counts.
func humanSizeGNU(n int64) string {
	if n == 0 {
		return "0"
	}
	suffixes := []string{"", "K", "M", "G", "T", "P", "E"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(suffixes)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d", n)
	}
	// GNU coreutils uses ceiling rounding (human_ceiling).
	if f >= 10 {
		return fmt.Sprintf("%d%s", int64(math.Ceil(f)), suffixes[i])
	}
	c := math.Ceil(f*10) / 10
	return fmt.Sprintf("%.1f%s", c, suffixes[i])
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
