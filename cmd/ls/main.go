// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1-R1.14, R2.1-R2.15, R3.1-R3.15: basic directory
// listing with single-column output (non-TTY default), dotfile filtering,
// multi-directory headers, mixed file/directory argument handling, error
// diagnostics, -1 single-column flag, -l long format with permissions/nlink/
// owner/group/size/mtime, file metadata via pkg/sys.Lstat, owner/group name
// resolution, -a (all entries including dotfiles), -A (almost all, excludes
// . and ..), -r (reverse sort), -R (recursive listing), -p (directory
// indicator), -C (multi-column vertical fill), -x (multi-column horizontal
// fill), format flag mutual exclusivity (last flag wins), -t (time sort),
// -S (size sort), -U (unsorted/directory order), -v (version sort),
// -i (inode display), -s (block count display), -n (numeric UID/GID,
// implies -l), -i and -s combined ordering, --color=always/auto/never with
// ANSI color output via pkg/format.
// R2.10: last sort flag wins. R3.1-R3.4: --color support.
// R3.5-R3.6: -h human-readable sizes in long format and total line.
// R3.7: -h with -s for human-readable block counts.
// R3.8-R3.10: -F classify indicator (/ * @ | =) with all output formats and color.
// R3.9: Executable defined as any execute bit set (mode&0o111 != 0).
// R3.10: -F works with -l, -1, -C, -x and color; indicator after reset sequence.
// R3.11: -R recursive listing with "PATH:" headers and blank-line separation.
// R3.12: -R respects current format mode (-l, -C, -x, default).
// R3.13: -R does not follow symbolic links to directories.
// R3.14: -R applies -a/-A filter flags to each subdirectory.
// R3.15: -R recurses subdirectories in the active sort order.
// Installs SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
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
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU ls format.
const progName = "ls"

// outputFormat controls the listing output mode.
type outputFormat int

const (
	formatDefault    outputFormat = iota // single-column when not TTY
	formatSingleCol                     // -1: one entry per line
	formatLong                          // -l: long format
	formatMultiCol                      // -C: multi-column, vertical fill
	formatHorizontal                    // -x: multi-column, horizontal fill
)

// sortMode controls the entry sort order.
// R2.10: When multiple sort flags are given, the last one wins.
type sortMode int

const (
	sortByName    sortMode = iota // default: C locale alphabetical
	sortByTime                    // -t: newest mtime first (R2.5)
	sortBySize                    // -S: largest first (R2.6)
	sortNone                      // -U: directory order (R2.8)
	sortByVersion                 // -v: version sort (R2.9)
)

// filterMode controls which entries are shown.
// R2.1/R2.2/R2.4: -a shows all, -A shows almost all, last flag wins.
type filterMode int

const (
	filterDefault   filterMode = iota // hide dotfiles
	filterAll                         // -a: show all including "." and ".."
	filterAlmostAll                   // -A: show dotfiles except "." and ".."
)

// colorMode controls colorized output.
// R3.1: --color=always/auto/never.
type colorMode int

const (
	colorNone   colorMode = iota // no --color flag given (default: no color)
	colorAlways                  // --color=always or --color without value
	colorAuto                    // --color=auto
	colorNever                   // --color=never
)

// lsOptions holds parsed command-line options.
type lsOptions struct {
	format    outputFormat
	sortBy    sortMode   // R2.5/R2.6/R2.8/R2.9/R2.10: sort mode
	filter    filterMode // R2.1/R2.2/R2.4: dotfile filter mode
	reverse   bool       // -r: reverse sort order
	recursive bool       // -R: list subdirectories recursively
	indicator bool       // -p: append '/' to directory names
	showInode  bool      // -i: prepend inode number (R2.11)
	showBlocks bool      // -s: prepend block count (R2.12)
	numericIDs    bool      // -n: display numeric UID/GID (R2.14)
	humanReadable bool      // -h: human-readable sizes (R3.5)
	classify      bool      // -F: append type indicator (R3.8)
	color         colorMode // R3.1: --color mode
	useColor      bool      // resolved: whether to colorize output (R3.1-R3.4)
}

func main() {
	// D1: Install SIGPIPE handler for clean pipe exit.
	sys.InstallSIGPIPEHandler()

	opts, paths := parseArgs(os.Args[1:])

	// R3.1-R3.4: Resolve color mode.
	switch opts.color {
	case colorAlways:
		format.SetColorEnabled(true)
		opts.useColor = true
	case colorAuto:
		// R3.2: Colorize only when stdout is a TTY.
		isTerminal := sys.IsTerminal(os.Stdout.Fd())
		format.SetColorEnabled(isTerminal)
		opts.useColor = isTerminal
	case colorNever:
		format.SetColorEnabled(false)
		opts.useColor = false
	default:
		// No --color flag given: no color output.
		opts.useColor = false
	}

	// R1.1/R1.2: Default to current directory when no arguments given.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0

	// R1.3: Separate file arguments from directory arguments. Files are
	// listed first, then directories, matching GNU ls argument ordering.
	var files []string
	var dirs []string

	for _, path := range paths {
		fi, err := os.Lstat(path)
		if err != nil {
			// R1.4: Print diagnostic to stderr for inaccessible arguments.
			fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, path, unwrapPathError(err))
			exitCode = 2
			continue
		}
		if fi.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
	}

	// R1.6/R1.7: For file arguments in long format, collect metadata and print.
	if opts.format == formatLong && len(files) > 0 {
		var entries []longEntry
		for _, f := range files {
			fi, err := sys.Lstat(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, f, unwrapPathError(err))
				exitCode = 2
				continue
			}
			displayName := f
			// R1.12: -p appends '/' to directory names.
			// R3.8: Skip -p when -F is active; -F handles indicators after color wrapping.
			if opts.indicator && !opts.classify && fi.Mode&os.ModeDir != 0 {
				displayName = f + "/"
			}
			// Note: color wrapping and -F indicator handled by printLongEntries.
			entries = append(entries, longEntry{name: displayName, path: f, fi: fi})
		}
		printLongEntries(entries, opts)
	} else if (opts.format == formatMultiCol || opts.format == formatHorizontal) && len(files) > 0 {
		// R1.13/R1.14: Multi-column output for file arguments.
		var displayNames []string
		for _, f := range files {
			displayName := f
			if opts.indicator || opts.useColor || opts.classify {
				fi, err := sys.Lstat(f)
				if err == nil {
					if opts.indicator && !opts.classify && fi.Mode&os.ModeDir != 0 {
						displayName = f + "/"
					}
					// R3.3: Colorize entry name.
					if opts.useColor {
						displayName = colorizeEntry(displayName, fi.Mode)
					}
					// R3.8/R3.10: Append classify indicator after color reset.
					if opts.classify {
						displayName += classifyIndicator(fi.Mode)
					}
				}
			}
			displayNames = append(displayNames, displayName)
		}
		tw := getTermWidth()
		if opts.format == formatMultiCol {
			printMultiCol(displayNames, tw)
		} else {
			printHorizontalCols(displayNames, tw)
		}
	} else {
		// R1.3: Print file arguments first, one per line.
		for _, f := range files {
			displayName := f
			if opts.indicator || opts.useColor || opts.classify {
				fi, err := sys.Lstat(f)
				if err == nil {
					if opts.indicator && !opts.classify && fi.Mode&os.ModeDir != 0 {
						displayName = f + "/"
					}
					// R3.3: Colorize entry name.
					if opts.useColor {
						displayName = colorizeEntry(displayName, fi.Mode)
					}
					// R3.8/R3.10: Append classify indicator after color reset.
					if opts.classify {
						displayName += classifyIndicator(fi.Mode)
					}
				}
			}
			fmt.Println(displayName)
		}
	}

	// R1.2/R3.11: When multiple directories, mix of files and directories,
	// or recursive mode, print each directory name as a header before its contents.
	needHeader := len(dirs) > 1 || (len(files) > 0 && len(dirs) > 0) || opts.recursive

	// R1.3: Blank line between file list and first directory when both present.
	needBlankBefore := len(files) > 0 && len(dirs) > 0

	for i, dir := range dirs {
		if needBlankBefore || (i > 0) {
			fmt.Println()
		}
		needBlankBefore = false

		if needHeader {
			fmt.Printf("%s:\n", dir)
		}

		if err := listDir(dir, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot open directory '%s': %v\n", progName, dir, unwrapPathError(err))
			exitCode = 2
		}
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into options and path operands.
// Flags are single-character and may be combined (e.g., -la). "--" ends flag
// parsing. Unknown flags cause exit 2 matching GNU ls.
// R1.5/R1.6: -1 sets single-column mode; -l sets long format.
// R1.13: -x sets horizontal multi-column mode.
// R1.14: -C, -x, -l, -1 are format flags; last flag wins (except -1 does
// not override -l, matching GNU ls behavior).
// R2.1: -a includes dotfiles. R2.2: -A almost all. R2.4: last -a/-A wins.
// R2.5: -t sorts by time. R2.6: -S sorts by size.
// R2.7: -r reverses sort. R2.8: -U unsorted. R2.10: last sort flag wins.
// R2.14: -n implies -l with numeric UID/GID.
// R3.11: -R lists recursively. R3.8 (partial): -p appends '/' to directories.
func parseArgs(args []string) (lsOptions, []string) {
	opts := lsOptions{format: formatDefault}
	var paths []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		// R3.1: Handle --color long option.
		if strings.HasPrefix(arg, "--") {
			if arg == "--color" || strings.HasPrefix(arg, "--color=") {
				val := "always" // R3.1: --color without value defaults to "always"
				if strings.HasPrefix(arg, "--color=") {
					val = arg[len("--color="):]
				}
				switch val {
				case "always":
					opts.color = colorAlways
				case "auto":
					opts.color = colorAuto
				case "never":
					opts.color = colorNever
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid argument '%s' for '--color'\n", progName, val)
					os.Exit(2)
				}
			} else {
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
				os.Exit(2)
			}
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case '1':
					// R1.5: -1 forces single-column output.
					// GNU ls: -1 does not override -l.
					if opts.format != formatLong {
						opts.format = formatSingleCol
					}
				case 'l':
					// R1.6: -l forces long format output.
					opts.format = formatLong
				case 'C':
					// R1.11/R1.14: -C forces multi-column vertical fill.
					opts.format = formatMultiCol
				case 'x':
					// R1.13/R1.14: -x forces multi-column horizontal fill.
					opts.format = formatHorizontal
				case 'a':
					// R2.1: -a includes all entries including "." and "..".
					opts.filter = filterAll
				case 'A':
					// R2.2: -A includes dotfiles except "." and "..".
					// R2.4: Last of -a/-A wins.
					opts.filter = filterAlmostAll
				case 't':
					// R2.5: -t sorts by modification time, newest first.
					opts.sortBy = sortByTime
				case 'S':
					// R2.6: -S sorts by file size, largest first.
					opts.sortBy = sortBySize
				case 'U':
					// R2.8: -U disables sorting (directory order).
					opts.sortBy = sortNone
				case 'v':
					// R2.9: -v sorts using version sort (strverscmp).
					opts.sortBy = sortByVersion
				case 'i':
					// R2.11: -i prepends inode number.
					opts.showInode = true
				case 'n':
					// R2.14: -n implies -l and displays numeric UID/GID.
					opts.numericIDs = true
					opts.format = formatLong
				case 's':
					// R2.12: -s prepends allocated block count.
					opts.showBlocks = true
				case 'h':
					// R3.5: -h enables human-readable sizes.
					opts.humanReadable = true
				case 'F':
					// R3.8: -F appends type indicator character.
					opts.classify = true
				case 'r':
					// R2.7: -r reverses sort order.
					opts.reverse = true
				case 'R':
					// R3.11: -R lists subdirectories recursively.
					opts.recursive = true
				case 'p':
					// R1.12: -p appends '/' to directory names.
					opts.indicator = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, ch)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(2)
				}
			}
			continue
		}
		paths = append(paths, arg)
	}

	return opts, paths
}

// defaultTermWidth is used when stdout is not a TTY (piped) and -C or -x is
// given, matching GNU ls behavior.
const defaultTermWidth = 80

// tabSize is the tab stop interval used by GNU ls for column alignment.
const tabSize = 8

// getTermWidth returns the terminal width for multi-column layout. Returns
// defaultTermWidth when stdout is not a TTY.
func getTermWidth() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return defaultTermWidth
	}
	return w
}

// writeIndent writes tabs and spaces to advance output from column position
// 'from' to column position 'to', matching the GNU ls indent algorithm.
// Returns the final column position.
func writeIndent(from, to int) int {
	for from < to {
		if tabSize > 0 && to/tabSize > from/tabSize {
			fmt.Print("\t")
			from += tabSize - from%tabSize
		} else {
			fmt.Print(" ")
			from++
		}
	}
	return from
}

// printMultiCol prints entries in multi-column vertical-fill layout using
// format.Columns. Entries fill top-to-bottom, left-to-right.
// R1.11/R1.12: -C multi-column vertical fill.
func printMultiCol(names []string, termWidth int) {
	if len(names) == 0 {
		return
	}

	rows := format.Columns(names, termWidth)
	if len(rows) == 0 {
		return
	}

	// Determine number of columns and compute per-column max widths.
	// R3.3: Use visibleWidth to ignore ANSI escape sequences in width calc.
	numCols := len(rows[0])
	colWidths := make([]int, numCols)
	for _, row := range rows {
		for c, entry := range row {
			w := visibleWidth(entry)
			if w > colWidths[c] {
				colWidths[c] = w
			}
		}
	}

	// Compute column start positions.
	const minGap = 2
	colStart := make([]int, numCols)
	pos := 0
	for c := range numCols {
		colStart[c] = pos
		pos += colWidths[c] + minGap
	}

	for _, row := range rows {
		curPos := 0
		for c, entry := range row {
			if c > 0 {
				curPos = writeIndent(curPos, colStart[c])
			}
			fmt.Print(entry)
			curPos += visibleWidth(entry)
		}
		fmt.Println()
	}
}

// printHorizontalCols prints entries in multi-column horizontal-fill layout.
// Entries fill left-to-right, top-to-bottom, matching GNU ls -x.
// R1.13: -x multi-column horizontal fill.
func printHorizontalCols(names []string, termWidth int) {
	if len(names) == 0 {
		return
	}

	// Precompute display widths.
	// R3.3: Use visibleWidth to ignore ANSI escape sequences.
	widths := make([]int, len(names))
	for i, name := range names {
		widths[i] = visibleWidth(name)
	}

	const minGap = 2

	// Try column counts from max down to 1 to find the widest layout that fits.
	bestCols := 1
	for numCols := len(names); numCols > 1; numCols-- {
		numRows := (len(names) + numCols - 1) / numCols

		totalWidth := 0
		fits := true
		for col := range numCols {
			colMax := 0
			for row := range numRows {
				idx := row*numCols + col // horizontal: row-major order
				if idx >= len(names) {
					continue
				}
				if widths[idx] > colMax {
					colMax = widths[idx]
				}
			}
			totalWidth += colMax
			if col < numCols-1 {
				totalWidth += minGap
			}
			if totalWidth > termWidth {
				fits = false
				break
			}
		}
		if fits {
			bestCols = numCols
			break
		}
	}

	numRows := (len(names) + bestCols - 1) / bestCols

	// Compute per-column widths for the chosen layout.
	colWidths := make([]int, bestCols)
	for col := range bestCols {
		for row := range numRows {
			idx := row*bestCols + col
			if idx >= len(names) {
				continue
			}
			if widths[idx] > colWidths[col] {
				colWidths[col] = widths[idx]
			}
		}
	}

	// Compute column start positions.
	colStart := make([]int, bestCols)
	pos := 0
	for c := range bestCols {
		colStart[c] = pos
		pos += colWidths[c] + minGap
	}

	// Print row by row, left to right.
	for row := range numRows {
		curPos := 0
		for col := range bestCols {
			idx := row*bestCols + col
			if idx >= len(names) {
				break
			}
			if col > 0 {
				curPos = writeIndent(curPos, colStart[col])
			}
			fmt.Print(names[idx])
			curPos += widths[idx]
		}
		fmt.Println()
	}
}

// longEntry holds the name and metadata for a single entry in long format.
type longEntry struct {
	name string // display name (basename for directory entries, full for file args)
	path string // full path for symlink resolution
	fi   *sys.FileInfo
}

// dirEntry pairs an entry name with its metadata for sorting.
// R2.5/R2.6: Used when -t or -S requires metadata-aware sorting.
type dirEntry struct {
	name string
	fi   *sys.FileInfo
}

// listDir reads and prints the contents of a single directory.
// R1.1/R1.2: One entry per line (non-TTY default), sorted alphabetically.
// R1.4: Entries whose names start with "." are excluded by default.
// R2.1: -a includes dotfiles including "." and "..".
// R2.5: -t sorts by modification time. R2.6: -S sorts by size.
// R2.7: -r reverses the current sort order.
// R2.8: -U lists in directory order (no sorting).
// R3.11-R3.15: -R recursively lists subdirectories.
// R1.12: -p appends '/' to directory names.
func listDir(path string, opts lsOptions) error {
	// R2.8: Use (*os.File).ReadDir to preserve directory order for -U.
	// os.ReadDir (package-level) always sorts; (*os.File).ReadDir does not.
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(-1)
	f.Close() // best-effort close; directory read is complete
	if err != nil {
		return err
	}

	// R2.8: When -U is given, preserve directory order (skip sorting).
	if opts.sortBy != sortNone {
		if opts.sortBy == sortByVersion {
			// R2.9: -v sorts using version sort (strverscmp semantics).
			sort.Slice(entries, func(i, j int) bool {
				return strverscmp(entries[i].Name(), entries[j].Name()) < 0
			})
		} else {
			// R1.3: Sort entries in C locale order (initial alphabetical sort).
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Name() < entries[j].Name()
			})
		}
	}

	// R1.4/R2.1/R2.2/R2.4/R3.14: Filter dotfiles based on -a/-A mode.
	var names []string
	if opts.filter == filterAll {
		// R2.1: -a includes "." and ".." as special entries.
		names = append(names, ".", "..")
	}
	for _, entry := range entries {
		name := entry.Name()
		if opts.filter == filterDefault && len(name) > 0 && name[0] == '.' {
			continue
		}
		names = append(names, name)
	}

	// R2.5/R2.6: When sorting by time or size, collect metadata and sort.
	if opts.sortBy == sortByTime || opts.sortBy == sortBySize {
		dirEntries := make([]dirEntry, 0, len(names))
		for _, name := range names {
			fullPath := filepath.Join(path, name)
			fi, err := sys.Lstat(fullPath)
			if err != nil {
				// Still include the entry; it will fail again when printing.
				dirEntries = append(dirEntries, dirEntry{name: name, fi: nil})
				continue
			}
			dirEntries = append(dirEntries, dirEntry{name: name, fi: fi})
		}

		switch opts.sortBy {
		case sortByTime:
			// R2.5: Sort by mtime newest first, tiebreak by name in C locale.
			sort.SliceStable(dirEntries, func(i, j int) bool {
				fi, fj := dirEntries[i].fi, dirEntries[j].fi
				if fi == nil || fj == nil {
					return dirEntries[i].name < dirEntries[j].name
				}
				if !fi.ModTime.Equal(fj.ModTime) {
					return fi.ModTime.After(fj.ModTime)
				}
				return dirEntries[i].name < dirEntries[j].name
			})
		case sortBySize:
			// R2.6: Sort by size largest first, tiebreak by name in C locale.
			sort.SliceStable(dirEntries, func(i, j int) bool {
				fi, fj := dirEntries[i].fi, dirEntries[j].fi
				if fi == nil || fj == nil {
					return dirEntries[i].name < dirEntries[j].name
				}
				if fi.Size != fj.Size {
					return fi.Size > fj.Size
				}
				return dirEntries[i].name < dirEntries[j].name
			})
		}

		// Rebuild names slice in sorted order.
		names = make([]string, len(dirEntries))
		for i, de := range dirEntries {
			names[i] = de.name
		}
	}

	// R2.7: Reverse sort order if -r is given.
	// R2.8: -r with -U has no defined effect; GNU ls does not reverse
	// directory order, so skip reversal when -U is active.
	if opts.reverse && opts.sortBy != sortNone {
		reverseStrings(names)
	}

	// R2.11/R2.12: When -i or -s is given, we need metadata for every entry
	// even in non-long formats. R3.3: Also needed when color is enabled.
	var metaMap map[string]*sys.FileInfo
	if opts.showInode || opts.showBlocks || opts.useColor || opts.classify {
		metaMap = make(map[string]*sys.FileInfo, len(names))
		for _, name := range names {
			fullPath := filepath.Join(path, name)
			fi, err := sys.Lstat(fullPath)
			if err == nil {
				metaMap[name] = fi
			}
		}
	}

	// R2.12/R2.13: When -s is active in non-long format, print "total N" line.
	// (Long format handles its own total line below.)
	if opts.showBlocks && opts.format != formatLong && metaMap != nil {
		var totalBlocks int64
		for _, name := range names {
			if fi := metaMap[name]; fi != nil {
				totalBlocks += fi.Blocks
			}
		}
		if opts.humanReadable {
			fmt.Printf("total %s\n", humanBlockSize(totalBlocks/2))
		} else {
			fmt.Printf("total %d\n", totalBlocks/2)
		}
	}

	// R1.6/R1.7: Long format requires metadata for each entry.
	if opts.format == formatLong {
		var longEntries []longEntry
		for _, name := range names {
			fullPath := filepath.Join(path, name)
			var fi *sys.FileInfo
			var err error
			if metaMap != nil {
				fi = metaMap[name]
				if fi == nil {
					_, err = sys.Lstat(fullPath)
				}
			} else {
				fi, err = sys.Lstat(fullPath)
			}
			if fi == nil {
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, fullPath, unwrapPathError(err))
				}
				continue
			}
			displayName := name
			// R1.12: -p appends '/' to directory names.
			// R3.8: Skip -p when -F is active; classify handles indicators.
			if opts.indicator && !opts.classify && fi.Mode&os.ModeDir != 0 {
				displayName = name + "/"
			}
			// Note: color wrapping and -F handled by printLongEntries.
			longEntries = append(longEntries, longEntry{name: displayName, path: fullPath, fi: fi})
		}
		// R1.10: Print "total N" block count line.
		var totalBlocks int64
		for _, le := range longEntries {
			totalBlocks += le.fi.Blocks
		}
		if opts.humanReadable {
			fmt.Printf("total %s\n", humanBlockSize(totalBlocks/2))
		} else {
			fmt.Printf("total %d\n", totalBlocks/2)
		}
		printLongEntries(longEntries, opts)
	} else if opts.format == formatMultiCol || opts.format == formatHorizontal {
		// R1.13/R1.14: Multi-column output.
		// R2.11/R2.12: Prepend inode/blocks prefix when -i or -s is active.
		var displayNames []string
		for _, name := range names {
			displayName := name
			if opts.indicator && !opts.classify {
				fullPath := filepath.Join(path, name)
				fi, err := os.Lstat(fullPath)
				if err == nil && fi.IsDir() {
					displayName = name + "/"
				}
			}
			// R3.3: Colorize entry name based on file type.
			if opts.useColor && metaMap != nil {
				if fi := metaMap[name]; fi != nil {
					displayName = colorizeEntry(displayName, fi.Mode)
				}
			}
			// R3.8/R3.10: Append classify indicator after color reset.
			if opts.classify && metaMap != nil {
				if fi := metaMap[name]; fi != nil {
					displayName += classifyIndicator(fi.Mode)
				}
			}
			displayName = prependMeta(displayName, name, metaMap, opts)
			displayNames = append(displayNames, displayName)
		}
		tw := getTermWidth()
		if opts.format == formatMultiCol {
			printMultiCol(displayNames, tw)
		} else {
			printHorizontalCols(displayNames, tw)
		}
	} else {
		// R1.1/R1.2/R1.5: Single-column output.
		// R2.11/R2.12: Compute column widths for inode/blocks alignment.
		maxIno, maxBlk := metaColumnWidths(names, metaMap, opts)
		for _, name := range names {
			displayName := name
			// R1.12: -p appends '/' to directory names.
			// R3.8: Skip -p when -F is active.
			if opts.indicator && !opts.classify {
				fullPath := filepath.Join(path, name)
				fi, err := os.Lstat(fullPath)
				if err == nil && fi.IsDir() {
					displayName = name + "/"
				}
			}
			// R3.3: Colorize entry name based on file type.
			if opts.useColor && metaMap != nil {
				if fi := metaMap[name]; fi != nil {
					displayName = colorizeEntry(displayName, fi.Mode)
				}
			}
			// R3.8/R3.10: Append classify indicator after color reset.
			if opts.classify && metaMap != nil {
				if fi := metaMap[name]; fi != nil {
					displayName += classifyIndicator(fi.Mode)
				}
			}
			prefix := metaPrefix(name, metaMap, opts, maxIno, maxBlk)
			fmt.Printf("%s%s\n", prefix, displayName)
		}
	}

	// R3.11-R3.15: Recursive listing of subdirectories.
	if opts.recursive {
		for _, name := range names {
			// R3.13: Do not recurse into "." or ".." to avoid infinite loops.
			if name == "." || name == ".." {
				continue
			}
			fullPath := filepath.Join(path, name)
			fi, err := os.Lstat(fullPath)
			if err != nil {
				continue
			}
			// R3.13: Only recurse into real directories, not symlinks.
			if fi.IsDir() {
				fmt.Printf("\n%s:\n", fullPath)
				if err := listDir(fullPath, opts); err != nil {
					fmt.Fprintf(os.Stderr, "%s: cannot open directory '%s': %v\n",
						progName, fullPath, unwrapPathError(err))
				}
			}
		}
	}

	return nil
}

// reverseStrings reverses a slice of strings in place.
func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// printLongEntries prints entries in long format with aligned columns.
// R1.6: permissions, nlink, owner, group, size, mtime, name.
// R1.7: metadata from sys.FileInfo via sys.Lstat.
// R1.8: owner/group name resolution via os/user.
// R2.11: -i prepends inode number. R2.12: -s prepends block count.
func printLongEntries(entries []longEntry, opts lsOptions) {
	if len(entries) == 0 {
		return
	}

	// Compute column widths for alignment.
	maxNlink := 0
	maxOwner := 0
	maxGroup := 0
	maxSize := 0
	maxIno := 0
	maxBlk := 0

	type resolvedEntry struct {
		ino   string // R2.11: inode number
		blk   string // R2.12: block count
		perms string
		nlink string
		owner string
		group string
		size  string
		mtime string
		name  string
	}

	resolved := make([]resolvedEntry, len(entries))
	for i, le := range entries {
		fi := le.fi
		// R2.14: -n displays numeric UID/GID instead of resolving names.
		var owner, group string
		if opts.numericIDs {
			owner = strconv.FormatUint(uint64(fi.Uid), 10)
			group = strconv.FormatUint(uint64(fi.Gid), 10)
		} else {
			owner = resolveOwner(fi.Uid)
			group = resolveGroup(fi.Gid)
		}
		re := resolvedEntry{
			perms: formatPermissions(fi.Mode),
			nlink: strconv.FormatUint(fi.Nlink, 10),
			owner: owner,
			group: group,
			size:  formatSize(fi.Size, opts.humanReadable),
			mtime: formatMtime(fi.ModTime),
			name:  le.name,
		}

		// R2.11: Format inode number.
		if opts.showInode {
			re.ino = strconv.FormatUint(fi.Ino, 10)
			if len(re.ino) > maxIno {
				maxIno = len(re.ino)
			}
		}

		// R2.12: Format block count in 1024-byte units.
		if opts.showBlocks {
			if opts.humanReadable {
				re.blk = humanBlockSize(fi.Blocks / 2)
			} else {
				re.blk = strconv.FormatInt(fi.Blocks/2, 10)
			}
			if len(re.blk) > maxBlk {
				maxBlk = len(re.blk)
			}
		}

		// R3.3: Colorize entry name based on file type.
		// R1.10: Symlink display â append " -> target".
		// R3.8/R3.10: Append classify indicator after color reset.
		if fi.Mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(le.path)
			if err == nil {
				colorizedName := le.name
				if opts.useColor {
					colorizedName = colorizeEntry(le.name, fi.Mode)
				}
					// R3.8: In long format, symlinks don't get '@' indicator
				// because " -> target" already indicates the symlink.
				re.name = colorizedName + " -> " + target
			}
		} else {
			displayName := le.name
			if opts.useColor {
				displayName = colorizeEntry(le.name, fi.Mode)
			}
			if opts.classify {
				displayName += classifyIndicator(fi.Mode)
			}
			re.name = displayName
		}

		if len(re.nlink) > maxNlink {
			maxNlink = len(re.nlink)
		}
		if len(re.owner) > maxOwner {
			maxOwner = len(re.owner)
		}
		if len(re.group) > maxGroup {
			maxGroup = len(re.group)
		}
		if len(re.size) > maxSize {
			maxSize = len(re.size)
		}

		resolved[i] = re
	}

	// R1.6: Print each entry with aligned fields.
	// R2.11/R2.12/R2.15: inode first, then blocks, then long-format fields.
	for _, re := range resolved {
		prefix := ""
		if opts.showInode {
			prefix += fmt.Sprintf("%*s ", maxIno, re.ino)
		}
		if opts.showBlocks {
			prefix += fmt.Sprintf("%*s ", maxBlk, re.blk)
		}
		fmt.Printf("%s%s %*s %-*s %-*s %*s %s %s\n",
			prefix,
			re.perms,
			maxNlink, re.nlink,
			maxOwner, re.owner,
			maxGroup, re.group,
			maxSize, re.size,
			re.mtime,
			re.name,
		)
	}
}

// formatPermissions produces the 10-character permission string for long format.
// R1.6: file type + owner rwx + group rwx + other rwx with setuid/setgid/sticky.
func formatPermissions(mode os.FileMode) string {
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

	// Positions 1-3: owner rwx.
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

	// Positions 4-6: group rwx.
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

	// Positions 7-9: other rwx.
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

// resolveOwner resolves a UID to a username, falling back to the numeric string.
// R1.8: Uses os/user.LookupId; falls back to numeric on error.
func resolveOwner(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup resolves a GID to a group name, falling back to the numeric string.
// R1.8: Uses os/user.LookupGroupId; falls back to numeric on error.
func resolveGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// formatMtime formats a modification time for long format display.
// R1.9: If within ~6 months, "Jan _2 15:04"; otherwise "Jan _2  2006".
func formatMtime(t time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// unwrapPathError extracts the inner error from *os.PathError for cleaner
// error messages matching GNU ls format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// colorStarted tracks whether the first colored entry has been output.
// GNU ls emits a reset code before the very first colored entry to
// initialize terminal color state.
var colorStarted bool

// colorizeEntry wraps an entry name with ANSI color codes based on file mode.
// R3.3: colorCode + name + resetCode. Returns name unchanged when the file
// type has no assigned color (regular file with no special bits).
// GNU ls emits a leading reset before the first colored entry.
func colorizeEntry(name string, mode os.FileMode) string {
	colorCode := format.FileTypeColor(mode)
	if colorCode == "" {
		return name
	}
	prefix := ""
	if !colorStarted {
		prefix = format.Reset()
		colorStarted = true
	}
	return prefix + colorCode + name + format.Reset()
}

// visibleWidth returns the visible character width of a string, ignoring
// ANSI escape sequences. Used for column layout when color is enabled.
func visibleWidth(s string) int {
	n := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip CSI escape sequence: ESC [ ... final_byte
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++ // skip 'm'
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		n++
	}
	return n
}

// metaColumnWidths computes the maximum column widths for inode and block
// count prefixes across all entries.
// R2.11/R2.12: Used for right-aligned prefix columns in non-long formats.
func metaColumnWidths(names []string, metaMap map[string]*sys.FileInfo, opts lsOptions) (maxIno, maxBlk int) {
	if metaMap == nil {
		return 0, 0
	}
	for _, name := range names {
		fi := metaMap[name]
		if fi == nil {
			continue
		}
		if opts.showInode {
			s := strconv.FormatUint(fi.Ino, 10)
			if len(s) > maxIno {
				maxIno = len(s)
			}
		}
		if opts.showBlocks {
			var s string
			if opts.humanReadable {
				s = humanBlockSize(fi.Blocks / 2)
			} else {
				s = strconv.FormatInt(fi.Blocks/2, 10)
			}
			if len(s) > maxBlk {
				maxBlk = len(s)
			}
		}
	}
	return maxIno, maxBlk
}

// metaPrefix returns the inode/blocks prefix string for a single entry in
// non-long output modes.
// R2.11/R2.12/R2.15: inode first, then blocks.
func metaPrefix(name string, metaMap map[string]*sys.FileInfo, opts lsOptions, maxIno, maxBlk int) string {
	if metaMap == nil {
		return ""
	}
	fi := metaMap[name]
	prefix := ""
	if opts.showInode {
		ino := "?"
		if fi != nil {
			ino = strconv.FormatUint(fi.Ino, 10)
		}
		prefix += fmt.Sprintf("%*s ", maxIno, ino)
	}
	if opts.showBlocks {
		blk := "?"
		if fi != nil {
			if opts.humanReadable {
				blk = humanBlockSize(fi.Blocks / 2)
			} else {
				blk = strconv.FormatInt(fi.Blocks/2, 10)
			}
		}
		prefix += fmt.Sprintf("%*s ", maxBlk, blk)
	}
	return prefix
}

// prependMeta prepends inode/blocks to a display name for multi-column modes.
// R2.11/R2.12: In multi-column output, the prefix is not separately aligned;
// it is part of the entry string that format.Columns lays out.
func prependMeta(displayName, origName string, metaMap map[string]*sys.FileInfo, opts lsOptions) string {
	if metaMap == nil {
		return displayName
	}
	fi := metaMap[origName]
	prefix := ""
	if opts.showInode {
		ino := "?"
		if fi != nil {
			ino = strconv.FormatUint(fi.Ino, 10)
		}
		prefix += ino + " "
	}
	if opts.showBlocks {
		blk := "?"
		if fi != nil {
			if opts.humanReadable {
				blk = humanBlockSize(fi.Blocks / 2)
			} else {
				blk = strconv.FormatInt(fi.Blocks/2, 10)
			}
		}
		prefix += blk + " "
	}
	return prefix + displayName
}

// formatSize formats a file size for long format display.
// R3.5: When humanReadable is true, use humanFileSize for GNU ls compatible output.
func formatSize(size int64, humanReadable bool) string {
	if humanReadable {
		return humanFileSize(size)
	}
	return strconv.FormatInt(size, 10)
}

// humanFileSize converts a byte count to a human-readable string matching
// GNU ls -h output format. Values < 1024 are shown as plain integers.
// Values >= 1024 use K/M/G/T/P/E suffixes with one decimal for values < 10.
// R3.5/R3.6: Used for file sizes and total line in long format.
func humanFileSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d", n)
	}
	suffixes := []string{"K", "M", "G", "T", "P", "E"}
	f := float64(n) / 1024
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

// humanBlockSize converts a block count (in 1024-byte units) to a
// human-readable string matching GNU ls -sh output format. Block counts
// are converted to bytes first, then formatted with humanFileSize.
// R3.7: Used for -s block counts and -l total line when -h is active.
func humanBlockSize(n int64) string {
	if n == 0 {
		return "0"
	}
	return humanFileSize(n * 1024)
}

// classifyIndicator returns the -F type indicator character for a file mode.
// R3.8: "/" for directories, "*" for executables, "@" for symlinks,
// "|" for FIFOs, "=" for sockets. Empty string for regular non-executable files.
// R3.9: Executable = any execute bit set (mode&0o111 != 0).
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
	case mode.IsRegular() && mode&0o111 != 0:
		return "*"
	default:
		return ""
	}
}

// strverscmp implements GNU strverscmp semantics for version sorting (R2.9).
// Runs of digits are compared numerically so "file2" < "file10".
func strverscmp(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := rune(a[ai]), rune(b[bi])
		aDigit := unicode.IsDigit(ca)
		bDigit := unicode.IsDigit(cb)

		if aDigit && bDigit {
			// Compare digit runs numerically.
			// Skip leading zeros — they affect ordering per strverscmp.
			aStart, bStart := ai, bi

			// Count leading zeros.
			aZeros := 0
			for ai < len(a) && a[ai] == '0' {
				aZeros++
				ai++
			}
			bZeros := 0
			for bi < len(b) && b[bi] == '0' {
				bZeros++
				bi++
			}

			// Collect remaining digits.
			aNumStart, bNumStart := ai, bi
			for ai < len(a) && unicode.IsDigit(rune(a[ai])) {
				ai++
			}
			for bi < len(b) && unicode.IsDigit(rune(b[bi])) {
				bi++
			}

			aLen := ai - aNumStart
			bLen := bi - bNumStart

			// If both runs are entirely zeros (or empty after stripping zeros).
			if aLen == 0 && bLen == 0 {
				// Pure zero runs: more zeros = smaller (strverscmp treats
				// leading-zero numbers as fractional-like, so "01" < "1").
				if aZeros != bZeros {
					// With leading zeros, the number with more leading zeros
					// sorts first if remaining digits are equal.
					_ = aStart
					_ = bStart
				}
			}

			// Different number of significant digits means different magnitude.
			if aLen != bLen {
				if aLen < bLen {
					return -1
				}
				return 1
			}

			// Same number of significant digits: compare lexicographically.
			for k := range aLen {
				if a[aNumStart+k] != b[bNumStart+k] {
					if a[aNumStart+k] < b[bNumStart+k] {
						return -1
					}
					return 1
				}
			}

			// Significant digits are equal. More leading zeros = smaller
			// (strverscmp: "01" < "1").
			if aZeros != bZeros {
				if aZeros > bZeros {
					return -1
				}
				return 1
			}

			continue
		}

		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
		ai++
		bi++
	}

	// Ran out of characters in one or both strings.
	if ai < len(a) {
		return 1
	}
	if bi < len(b) {
		return -1
	}
	return 0
}
