// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU ls: list directory contents.
// Implements prd008-ls R1-R7 (basic listing, long format, filter flags, color,
// human-readable sizes, exit codes, signal handling) and prd010-ls-extended
// R1-R6 (multi-column control, sort order, metadata display, classification,
// recursive listing, flag interactions).
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

// formatMode represents the output format selected by format flags.
type formatMode int

const (
	formatDefault    formatMode = iota // multi-column if TTY, single-column otherwise
	formatLong                         // -l or -n
	formatSingleCol                    // -1
	formatColumns                      // -C
	formatAcross                       // -x
)

// sortMode represents the active sort key.
type sortMode int

const (
	sortName    sortMode = iota // default: C locale alphabetical
	sortTime                    // -t: modification time, newest first
	sortSize                    // -S: file size, largest first
	sortNone                    // -U: directory order (no sorting)
	sortVersion                 // -v: version sort (strverscmp)
)

// options holds parsed command-line flags.
type options struct {
	showAll     bool       // -a: include dotfiles including . and ..
	almostAll   bool       // -A: include dotfiles excluding . and ..
	format      formatMode // output format (last format flag wins)
	recursive   bool       // -R: recursive listing
	dirOnly     bool       // -d: list directory entries themselves
	humanSize   bool       // -h: human-readable sizes with -l
	sortBy      sortMode   // active sort key (last sort flag wins)
	reverseSort bool       // -r: reverse sort order
	colorMode   string     // "always", "auto", "never"
	showInode   bool       // -i: prepend inode number
	showBlocks  bool       // -s: prepend block count
	numericIDs  bool       // -n: numeric UID/GID (implies long format)
	classify    bool       // -F: append type indicator
}

func main() {
	// D1: SIGPIPE handler per ARCHITECTURE.yaml.
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	// R6.1: -n implies -l.
	if opts.numericIDs {
		opts.format = formatLong
	}

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

	termWidth := 80
	if isTTY {
		if w, err := sys.TerminalWidth(); err == nil && w > 0 {
			termWidth = w
		}
	}

	// Resolve the effective format mode.
	useAcross := false
	singleColumn := false
	longFormat := false

	switch opts.format {
	case formatDefault:
		if !isTTY {
			singleColumn = true
		}
	case formatLong:
		longFormat = true
	case formatSingleCol:
		singleColumn = true
	case formatColumns:
		// Multi-column vertical is the default when not singleColumn/longFormat/useAcross.
	case formatAcross:
		useAcross = true
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

	sortPaths(fileArgs, opts)
	sortPaths(dirArgs, opts)

	needBlank := false

	// R2: List file arguments first.
	if len(fileArgs) > 0 {
		if longFormat {
			printLongFiles(fileArgs, opts)
		} else if singleColumn {
			printSingleColFiles(fileArgs, fileArgs, opts)
		} else {
			printMultiColFiles(fileArgs, fileArgs, termWidth, useAcross, opts)
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
		if err := listDir(dir, opts, longFormat, singleColumn, useAcross, termWidth); err != nil {
			exitCode = 1
		}
		if i < len(dirArgs)-1 {
			needBlank = true
		}
	}

	// R5: -R recursive listing.
	if opts.recursive && len(dirArgs) > 0 {
		for _, dir := range dirArgs {
			if err := listRecursive(dir, opts, longFormat, singleColumn, useAcross, termWidth); err != nil {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

// listDir lists the contents of a single directory.
func listDir(dir string, opts options, longFormat, singleColumn, useAcross bool, termWidth int) error {
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

	// R1.3: Sort in C locale order; sort flags override.
	sortEntries(names, dir, opts)

	if longFormat {
		printLongDir(dir, names, opts)
	} else if singleColumn {
		paths := make([]string, len(names))
		for i, name := range names {
			paths[i] = filepath.Join(dir, name)
		}
		printSingleColFiles(names, paths, opts)
	} else {
		paths := make([]string, len(names))
		for i, name := range names {
			paths[i] = filepath.Join(dir, name)
		}
		printMultiColFiles(names, paths, termWidth, useAcross, opts)
	}
	return nil
}

// listRecursive recursively lists subdirectories (R5 -R).
func listRecursive(dir string, opts options, longFormat, singleColumn, useAcross bool, termWidth int) error {
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
		// R5.3: Do not follow symbolic links to directories.
		if e.IsDir() && name != "." && name != ".." {
			subdirs = append(subdirs, name)
		}
	}
	sortEntries(subdirs, dir, opts)

	var retErr error
	for _, sub := range subdirs {
		subPath := filepath.Join(dir, sub)
		fmt.Printf("\n%s:\n", subPath)
		if err := listDir(subPath, opts, longFormat, singleColumn, useAcross, termWidth); err != nil {
			retErr = err
		}
		if err := listRecursive(subPath, opts, longFormat, singleColumn, useAcross, termWidth); err != nil {
			retErr = err
		}
	}
	return retErr
}

// sortPaths sorts a slice of file paths in place according to the sort flags.
func sortPaths(paths []string, opts options) {
	if opts.sortBy == sortNone {
		return
	}
	if opts.sortBy == sortVersion {
		sort.SliceStable(paths, func(i, j int) bool {
			less := versionCompare(paths[i], paths[j]) < 0
			if opts.reverseSort {
				return !less
			}
			return less
		})
		return
	}
	if opts.sortBy == sortName {
		sort.Strings(paths)
		if opts.reverseSort {
			reverseSlice(paths)
		}
		return
	}
	// sortByTime or sortBySize: gather FileInfo.
	infos := make(map[string]*sys.FileInfo, len(paths))
	for _, p := range paths {
		fi, err := sys.Lstat(p)
		if err == nil {
			infos[p] = fi
		}
	}
	sort.SliceStable(paths, func(i, j int) bool {
		less := comparePaths(paths[i], paths[j], infos, opts)
		if opts.reverseSort {
			return !less
		}
		return less
	})
}

// sortEntries sorts entry names within a directory according to sort flags.
func sortEntries(names []string, dir string, opts options) {
	if opts.sortBy == sortNone {
		return
	}
	if opts.sortBy == sortVersion {
		sort.SliceStable(names, func(i, j int) bool {
			less := versionCompare(names[i], names[j]) < 0
			if opts.reverseSort {
				return !less
			}
			return less
		})
		return
	}
	if opts.sortBy == sortName {
		sort.Strings(names)
		if opts.reverseSort {
			reverseSlice(names)
		}
		return
	}
	infos := make(map[string]*sys.FileInfo, len(names))
	for _, name := range names {
		fi, err := sys.Lstat(filepath.Join(dir, name))
		if err == nil {
			infos[name] = fi
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		less := compareEntries(names[i], names[j], infos, opts)
		if opts.reverseSort {
			return !less
		}
		return less
	})
}

// comparePaths compares two paths for sorting by -t or -S, falling back to name.
func comparePaths(a, b string, infos map[string]*sys.FileInfo, opts options) bool {
	fia, aOK := infos[a]
	fib, bOK := infos[b]
	if opts.sortBy == sortTime && aOK && bOK {
		if !fia.ModTime.Equal(fib.ModTime) {
			return fia.ModTime.After(fib.ModTime)
		}
	}
	if opts.sortBy == sortSize && aOK && bOK {
		if fia.Size != fib.Size {
			return fia.Size > fib.Size
		}
	}
	return a < b
}

// compareEntries compares two entry names for sorting by -t or -S, falling back to name.
func compareEntries(a, b string, infos map[string]*sys.FileInfo, opts options) bool {
	fia, aOK := infos[a]
	fib, bOK := infos[b]
	if opts.sortBy == sortTime && aOK && bOK {
		if !fia.ModTime.Equal(fib.ModTime) {
			return fia.ModTime.After(fib.ModTime)
		}
	}
	if opts.sortBy == sortSize && aOK && bOK {
		if fia.Size != fib.Size {
			return fia.Size > fib.Size
		}
	}
	return a < b
}

// reverseSlice reverses a string slice in place.
func reverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// classifyIndicator returns the -F type indicator for a file mode. R4.1.
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

// printSingleColFiles prints entries in single-column format with optional -i/-s/-F.
func printSingleColFiles(names []string, paths []string, opts options) {
	// Gather metadata for -i/-s prefix widths.
	type metaEntry struct {
		name  string
		path  string
		fi    *sys.FileInfo
		ino   string
		blk   string
	}
	entries := make([]metaEntry, len(names))
	inodeW, blockW := 0, 0
	for i, name := range names {
		entries[i].name = name
		entries[i].path = paths[i]
		fi, err := sys.Lstat(paths[i])
		if err == nil {
			entries[i].fi = fi
		}
		if opts.showInode && fi != nil {
			s := strconv.FormatUint(fi.Ino, 10)
			entries[i].ino = s
			if len(s) > inodeW {
				inodeW = len(s)
			}
		}
		if opts.showBlocks && fi != nil {
			blk := fi.Blocks / 2
			var s string
			if opts.humanSize {
				s = blockHumanSize(blk)
			} else {
				s = strconv.FormatInt(blk, 10)
			}
			entries[i].blk = s
			if len(s) > blockW {
				blockW = len(s)
			}
		}
	}

	// Print total line when -s is active.
	if opts.showBlocks {
		var totalBlocks int64
		for _, e := range entries {
			if e.fi != nil {
				totalBlocks += e.fi.Blocks
			}
		}
		totalK := totalBlocks / 2
		if opts.humanSize {
			fmt.Printf("total %s\n", blockHumanSize(totalK))
		} else {
			fmt.Printf("total %d\n", totalK)
		}
	}

	for _, e := range entries {
		if opts.showInode {
			fmt.Printf("%*s ", inodeW, e.ino)
		}
		if opts.showBlocks {
			fmt.Printf("%*s ", blockW, e.blk)
		}
		printColorized(e.name, e.path)
		if opts.classify && e.fi != nil {
			fmt.Print(classifyIndicator(e.fi.Mode))
		}
		fmt.Println()
	}
}

// printMultiColFiles prints entries in multi-column format with optional -i/-s/-F.
func printMultiColFiles(names []string, paths []string, termWidth int, across bool, opts options) {
	// Build display strings incorporating -i/-s prefixes and -F suffixes.
	type metaEntry struct {
		fi  *sys.FileInfo
		ino string
		blk string
	}
	metas := make([]metaEntry, len(names))
	inodeW, blockW := 0, 0
	for i := range names {
		fi, err := sys.Lstat(paths[i])
		if err == nil {
			metas[i].fi = fi
		}
		if opts.showInode && fi != nil {
			s := strconv.FormatUint(fi.Ino, 10)
			metas[i].ino = s
			if len(s) > inodeW {
				inodeW = len(s)
			}
		}
		if opts.showBlocks && fi != nil {
			blk := fi.Blocks / 2
			var s string
			if opts.humanSize {
				s = blockHumanSize(blk)
			} else {
				s = strconv.FormatInt(blk, 10)
			}
			metas[i].blk = s
			if len(s) > blockW {
				blockW = len(s)
			}
		}
	}

	// Print total line when -s is active.
	if opts.showBlocks {
		type hasBlocks struct {
			fi *sys.FileInfo
		}
		var totalBlocks int64
		for _, m := range metas {
			if m.fi != nil {
				totalBlocks += m.fi.Blocks
			}
		}
		totalK := totalBlocks / 2
		if opts.humanSize {
			fmt.Printf("total %s\n", blockHumanSize(totalK))
		} else {
			fmt.Printf("total %d\n", totalK)
		}
	}

	// Build "plain" names for column width calculation, including prefix/suffix widths.
	plainNames := make([]string, len(names))
	for i, name := range names {
		var prefix string
		if opts.showInode {
			prefix += fmt.Sprintf("%*s ", inodeW, metas[i].ino)
		}
		if opts.showBlocks {
			prefix += fmt.Sprintf("%*s ", blockW, metas[i].blk)
		}
		suffix := ""
		if opts.classify && metas[i].fi != nil {
			suffix = classifyIndicator(metas[i].fi.Mode)
		}
		plainNames[i] = prefix + name + suffix
	}

	// Use Columns for vertical layout, or ColumnsAcross for horizontal.
	var rows [][]string
	if across {
		rows = columnsAcross(plainNames, termWidth)
	} else {
		rows = format.Columns(plainNames, termWidth)
	}

	// Build colorized display names.
	displayNames := make([]string, len(plainNames))
	for i, name := range names {
		var prefix string
		if opts.showInode {
			prefix += fmt.Sprintf("%*s ", inodeW, metas[i].ino)
		}
		if opts.showBlocks {
			prefix += fmt.Sprintf("%*s ", blockW, metas[i].blk)
		}
		color := ""
		reset := ""
		if metas[i].fi != nil {
			color = format.FileTypeColor(metas[i].fi.Mode)
			reset = format.Reset()
		}
		suffix := ""
		if opts.classify && metas[i].fi != nil {
			suffix = classifyIndicator(metas[i].fi.Mode)
		}
		displayNames[i] = prefix + color + name + reset + suffix
	}

	// Map plain names to indices for display lookup.
	plainToIdx := make(map[string][]int, len(plainNames))
	for i, pn := range plainNames {
		plainToIdx[pn] = append(plainToIdx[pn], i)
	}
	used := make(map[int]bool, len(plainNames))

	for _, row := range rows {
		for j, entry := range row {
			// Find index for this entry.
			idx := -1
			for _, candidate := range plainToIdx[entry] {
				if !used[candidate] {
					idx = candidate
					used[candidate] = true
					break
				}
			}
			display := entry
			if idx >= 0 {
				display = displayNames[idx]
			}
			if j < len(row)-1 {
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

// columnsAcross arranges entries across rows then down (for -x flag). R1.3.
func columnsAcross(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}

	// Try maximizing column count.
	for numCols := n; numCols >= 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		// Compute per-column widths from the longest entry in each column.
		// In across mode, entries fill left-to-right: idx = row*numCols + col.
		colWidths := make([]int, numCols)
		for row := 0; row < numRows; row++ {
			for col := 0; col < numCols; col++ {
				idx := row*numCols + col
				if idx >= n {
					continue
				}
				w := utf8.RuneCountInString(entries[idx])
				if w > colWidths[col] {
					colWidths[col] = w
				}
			}
		}

		totalWidth := 0
		for i, cw := range colWidths {
			totalWidth += cw
			if i < numCols-1 {
				totalWidth += 2
			}
		}
		if totalWidth > termWidth && numCols > 1 {
			continue
		}

		rows := make([][]string, numRows)
		for row := 0; row < numRows; row++ {
			var rowEntries []string
			for col := 0; col < numCols; col++ {
				idx := row*numCols + col
				if idx >= n {
					break
				}
				rowEntries = append(rowEntries, entries[idx])
			}
			rows[row] = rowEntries
		}
		return rows
	}

	// Fallback: single column.
	rows := make([][]string, n)
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
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
		infos = append(infos, entryInfo{name: f, fi: fi, path: f})
	}
	if len(infos) == 0 {
		return
	}

	// Print -s total if active.
	if opts.showBlocks {
		var totalBlocks int64
		for _, e := range infos {
			totalBlocks += e.fi.Blocks
		}
		totalK := totalBlocks / 2
		if opts.humanSize {
			fmt.Printf("total %s\n", blockHumanSize(totalK))
		} else {
			fmt.Printf("total %d\n", totalK)
		}
	}

	inodeW, blockW := computeMetaWidths(infos, opts)
	nlinkW, ownerW, groupW, sizeW := computeFieldWidths(infos, opts)
	for _, e := range infos {
		printLongEntry(e.name, e.fi, e.path, opts, inodeW, blockW, nlinkW, ownerW, groupW, sizeW)
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
		fmt.Printf("total %s\n", blockHumanSize(totalK))
	} else {
		fmt.Printf("total %d\n", totalK)
	}

	if len(infos) == 0 {
		return
	}

	inodeW, blockW := computeMetaWidths(infos, opts)
	nlinkW, ownerW, groupW, sizeW := computeFieldWidths(infos, opts)
	for _, e := range infos {
		printLongEntry(e.name, e.fi, e.path, opts, inodeW, blockW, nlinkW, ownerW, groupW, sizeW)
	}
}

// computeMetaWidths calculates column widths for -i inode and -s block prefix fields.
func computeMetaWidths(infos []entryInfo, opts options) (inodeW, blockW int) {
	for _, e := range infos {
		if opts.showInode {
			w := len(strconv.FormatUint(e.fi.Ino, 10))
			if w > inodeW {
				inodeW = w
			}
		}
		if opts.showBlocks {
			blk := e.fi.Blocks / 2
			var s string
			if opts.humanSize {
				s = blockHumanSize(blk)
			} else {
				s = strconv.FormatInt(blk, 10)
			}
			if len(s) > blockW {
				blockW = len(s)
			}
		}
	}
	return
}

// computeFieldWidths calculates column widths for long format fields.
func computeFieldWidths(infos []entryInfo, opts options) (nlinkW, ownerW, groupW, sizeW int) {
	for _, e := range infos {
		nw := len(strconv.FormatUint(e.fi.Nlink, 10))
		if nw > nlinkW {
			nlinkW = nw
		}
		var ow int
		if opts.numericIDs {
			ow = len(strconv.FormatUint(uint64(e.fi.Uid), 10))
		} else {
			ow = len(lookupUser(e.fi.Uid))
		}
		if ow > ownerW {
			ownerW = ow
		}
		var gw int
		if opts.numericIDs {
			gw = len(strconv.FormatUint(uint64(e.fi.Gid), 10))
		} else {
			gw = len(lookupGroup(e.fi.Gid))
		}
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
func printLongEntry(name string, fi *sys.FileInfo, path string, opts options, inodeW, blockW, nlinkW, ownerW, groupW, sizeW int) {
	// R3.1: -i inode prefix.
	if opts.showInode {
		fmt.Printf("%*s ", inodeW, strconv.FormatUint(fi.Ino, 10))
	}
	// R3.2: -s block count prefix.
	if opts.showBlocks {
		blk := fi.Blocks / 2
		var s string
		if opts.humanSize {
			s = blockHumanSize(blk)
		} else {
			s = strconv.FormatInt(blk, 10)
		}
		fmt.Printf("%*s ", blockW, s)
	}

	perms := permString(fi.Mode)
	nlink := strconv.FormatUint(fi.Nlink, 10)

	// R3.4: -n numeric UID/GID.
	var owner, group string
	if opts.numericIDs {
		owner = strconv.FormatUint(uint64(fi.Uid), 10)
		group = strconv.FormatUint(uint64(fi.Gid), 10)
	} else {
		owner = lookupUser(fi.Uid)
		group = lookupGroup(fi.Gid)
	}

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

	// R4.3: -F indicator after reset sequence.
	if opts.classify {
		displayName += classifyIndicator(fi.Mode)
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

// blockHumanSize converts 1K-block count to human-readable form. R6.5.
func blockHumanSize(n int64) string {
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
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d%s", int64(f), suffixes[i])
	}
	return fmt.Sprintf("%.1f%s", f, suffixes[i])
}

// versionCompare implements strverscmp-like comparison for -v sort. R2.5.
// Returns negative if a < b, 0 if equal, positive if a > b.
func versionCompare(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := rune(a[ai]), rune(b[bi])
		aDigit := unicode.IsDigit(ca)
		bDigit := unicode.IsDigit(cb)

		if aDigit && bDigit {
			// Compare numeric runs.
			aStart, bStart := ai, bi
			for ai < len(a) && unicode.IsDigit(rune(a[ai])) {
				ai++
			}
			for bi < len(b) && unicode.IsDigit(rune(b[bi])) {
				bi++
			}
			aNum := a[aStart:ai]
			bNum := b[bStart:bi]

			// Strip leading zeros for numeric comparison.
			aStripped := strings.TrimLeft(aNum, "0")
			bStripped := strings.TrimLeft(bNum, "0")
			if aStripped == "" {
				aStripped = "0"
			}
			if bStripped == "" {
				bStripped = "0"
			}

			// Compare by length first (longer = bigger), then lexicographically.
			if len(aStripped) != len(bStripped) {
				return len(aStripped) - len(bStripped)
			}
			if aStripped != bStripped {
				if aStripped < bStripped {
					return -1
				}
				return 1
			}
			// Equal numeric value; compare by number of leading zeros (fewer zeros = smaller).
			if len(aNum) != len(bNum) {
				return len(aNum) - len(bNum)
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

	return len(a) - len(b)
}

// parseArgs parses ls command-line flags. R4, R6.3.
func parseArgs(args []string) (options, []string) {
	opts := options{colorMode: "auto", sortBy: sortName, format: formatDefault}
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
				// Format flags: last one wins (R1.4, R6.2).
				case '1':
					opts.format = formatSingleCol
				case 'l':
					opts.format = formatLong
				case 'C':
					opts.format = formatColumns
				case 'x':
					opts.format = formatAcross
				case 'R':
					opts.recursive = true
				case 'd':
					opts.dirOnly = true
				case 'h':
					opts.humanSize = true
				// Sort flags: last one wins (R2.6).
				case 't':
					opts.sortBy = sortTime
				case 'S':
					opts.sortBy = sortSize
				case 'U':
					opts.sortBy = sortNone
				case 'v':
					opts.sortBy = sortVersion
				case 'r':
					opts.reverseSort = true
				// Metadata display flags.
				case 'i':
					opts.showInode = true
				case 's':
					opts.showBlocks = true
				case 'n':
					opts.numericIDs = true
				case 'F':
					opts.classify = true
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
	fmt.Println("  -C                         list entries by columns")
	fmt.Println("  -d, --directory            list directories themselves, not their contents")
	fmt.Println("  -F, --classify             append indicator (one of */=>@|) to entries")
	fmt.Println("  -h, --human-readable       with -l, print sizes like 1K 234M 2G etc.")
	fmt.Println("  -i, --inode                print the index number of each file")
	fmt.Println("  -l                         use a long listing format")
	fmt.Println("  -n, --numeric-uid-gid      like -l, but list numeric user and group IDs")
	fmt.Println("  -r, --reverse              reverse order while sorting")
	fmt.Println("  -R, --recursive            list subdirectories recursively")
	fmt.Println("  -s, --size                 print the allocated size of each file, in blocks")
	fmt.Println("  -S                         sort by file size, largest first")
	fmt.Println("  -t                         sort by time, newest first")
	fmt.Println("  -U                         do not sort; list entries in directory order")
	fmt.Println("  -v                         natural sort of (version) numbers within text")
	fmt.Println("  -x                         list entries by lines instead of by columns")
	fmt.Println("  -1                         list one file per line")
	fmt.Println("      --color[=WHEN]         colorize the output; WHEN can be 'always',")
	fmt.Println("                               'auto', or 'never'; more info below")
	fmt.Println("      --help        display this help and exit")
	fmt.Println("      --version     output version information and exit")
}
