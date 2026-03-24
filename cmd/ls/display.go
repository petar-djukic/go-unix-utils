// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls display logic: long format output, columnar layout,
// permission strings, inode/block prefix display, human-readable sizes,
// color wrapping, time formatting, and symlink display.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultTermWidth is used when stdout is not a TTY and -C is forced.
const defaultTermWidth = 80

// sixMonths approximates 6 months for mtime formatting cutoff.
const sixMonths = 6 * 30 * 24 * time.Hour

// prefixWidths holds column widths for -i and -s prefix columns.
type prefixWidths struct {
	inode  int
	blocks int
}

// longWidths holds column widths for long-format alignment.
type longWidths struct {
	nlink int
	owner int
	group int
	size  int
}

// displayEntries outputs file-argument entries (no total line).
func displayEntries(entries []entry, cfg lsConfig) int {
	if cfg.output == modeLong {
		return printLongEntries(entries, cfg)
	}
	printNonLongEntries(entries, cfg)
	return 0
}

// displayDirEntries outputs directory entries with total line when needed.
func displayDirEntries(entries []entry, cfg lsConfig) int {
	if cfg.output == modeLong {
		return printLongDir(entries, cfg)
	}
	// R2.12: -s displays a total line for directory listings.
	if cfg.showBlocks {
		printTotalLine(entries, cfg)
	}
	printNonLongEntries(entries, cfg)
	return 0
}

// printTotalLine prints the "total N" line for -l or -s.
// R1.10: total = sum(Blocks)/2 in 1K-block units.
// R3.6: -h converts total to human-readable form.
func printTotalLine(entries []entry, cfg lsConfig) {
	var totalBlocks int64
	for _, e := range entries {
		if e.info != nil {
			totalBlocks += e.info.Blocks
		}
	}
	kBlocks := totalBlocks / 2
	if cfg.humanReadable {
		// R3.6: convert 1K-blocks to bytes for HumanSize.
		fmt.Printf("total %s\n", format.HumanSize(kBlocks*1024, format.HumanSizeOpts{Binary: true}))
	} else {
		fmt.Printf("total %d\n", kBlocks)
	}
}

// printNonLongEntries displays entries in single-column or columnar format
// with optional -i and -s prefixes.
func printNonLongEntries(entries []entry, cfg lsConfig) {
	if len(entries) == 0 {
		return
	}
	pw := computePrefixWidths(entries, cfg)
	names := buildDisplayNames(entries, cfg, pw)
	mode := resolveOutputMode(cfg)
	switch mode {
	case modeColumns:
		printColumnar(names)
		return
	case modeHorizontal:
		printHorizontalColumnar(names)
		return
	case modeComma:
		printCommaEntries(names)
		return
	}
	for _, name := range names {
		fmt.Println(name)
	}
}

// buildDisplayNames constructs display strings with -i/-s prefixes.
func buildDisplayNames(entries []entry, cfg lsConfig, pw prefixWidths) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = buildDisplayName(e, cfg, pw)
	}
	return names
}

// buildDisplayName constructs a single display string with optional prefixes.
// R2.15: when both -i and -s are given, inode is first, then block count.
// R3.3: wraps entry name in ANSI color codes when color is active.
// R3.10: indicator is printed after the color reset sequence.
func buildDisplayName(e entry, cfg lsConfig, pw prefixWidths) string {
	var parts []string
	if cfg.showInode {
		parts = append(parts, format.PadLeft(inodeString(e), pw.inode))
	}
	if cfg.showBlocks {
		parts = append(parts, format.PadLeft(blockString(e, cfg.humanReadable), pw.blocks))
	}
	name := coloredEntryName(e.name, e.info)
	parts = append(parts, name+typeIndicator(e.info, cfg.indicator))
	return strings.Join(parts, " ")
}

// coloredEntryName wraps a name in ANSI color codes based on file type.
// R3.3: uses pkg/format.FileTypeColor and Reset.
// R3.4: returns plain name when color is disabled.
func coloredEntryName(name string, info *sys.FileInfo) string {
	if info == nil {
		return name
	}
	c := format.FileTypeColor(info.Mode)
	if c == "" {
		return name
	}
	return c + name + format.Reset()
}

// computePrefixWidths calculates column widths for -i and -s prefixes.
func computePrefixWidths(entries []entry, cfg lsConfig) prefixWidths {
	var pw prefixWidths
	for _, e := range entries {
		if cfg.showInode {
			updateWidth(&pw.inode, len(inodeString(e)))
		}
		if cfg.showBlocks {
			updateWidth(&pw.blocks, len(blockString(e, cfg.humanReadable)))
		}
	}
	return pw
}

// inodeString returns the inode number as a string.
// R2.11: uses sys.FileInfo.Ino.
func inodeString(e entry) string {
	if e.info == nil {
		return "?"
	}
	return strconv.FormatUint(e.info.Ino, 10)
}

// blockString returns the block count in 1K-block units as a string.
// R2.12: fi.Blocks / 2 converts 512-byte blocks to 1K blocks.
// R3.7: -h converts block counts to human-readable form.
func blockString(e entry, humanReadable bool) string {
	if e.info == nil {
		return "?"
	}
	kBlocks := e.info.Blocks / 2
	if humanReadable {
		return format.HumanSize(kBlocks*1024, format.HumanSizeOpts{Binary: true})
	}
	return strconv.FormatInt(kBlocks, 10)
}

// resolveOutputMode determines the effective output mode.
// R1.1/R1.2: default is multi-column when TTY, single-column otherwise.
// R1.5: -1 forces single-column. R1.11: -C forces multi-column.
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
		for col, e := range row {
			w := utf8.RuneCountInString(e)
			if w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// printColumnarRow prints a single row of multi-column output.
func printColumnarRow(row []string, colWidths []int) {
	for i, e := range row {
		if i < len(row)-1 {
			fmt.Print(format.PadRight(e, colWidths[i]+2))
		} else {
			fmt.Print(e)
		}
	}
	fmt.Println()
}

// printLongDir prints directory entries in long format with total line.
// R1.10: "total N" block count line precedes directory listing.
func printLongDir(entries []entry, cfg lsConfig) int {
	printTotalLine(entries, cfg)
	return printLongLines(entries, cfg)
}

// printLongEntries prints file-argument entries in long format (no total line).
func printLongEntries(entries []entry, cfg lsConfig) int {
	return printLongLines(entries, cfg)
}

// printLongLines prints long-format output with aligned columns.
func printLongLines(entries []entry, cfg lsConfig) int {
	exitCode := 0
	sf := sys.Lstat
	if cfg.derefAll {
		sf = sys.Stat
	}
	for i := range entries {
		if entries[i].info == nil {
			fi, err := sf(entries[i].path)
			if err != nil {
				reportError("cannot access", entries[i].name, err)
				exitCode = 1
				continue
			}
			entries[i].info = fi
		}
	}
	lw := computeLongWidths(entries, cfg)
	pw := computePrefixWidths(entries, cfg)
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		printLongLine(e, cfg, lw, pw)
	}
	return exitCode
}

// computeLongWidths calculates column widths for long-format output.
func computeLongWidths(entries []entry, cfg lsConfig) longWidths {
	var w longWidths
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		updateWidth(&w.nlink, len(strconv.FormatUint(e.info.Nlink, 10)))
		updateWidth(&w.owner, len(ownerString(e.info.Uid, cfg.numericIDs)))
		updateWidth(&w.group, len(groupString(e.info.Gid, cfg.numericIDs)))
		updateWidth(&w.size, len(sizeString(e.info.Size, cfg.humanReadable)))
	}
	return w
}

// printLongLine prints one entry in long format.
// R1.6: [inode] [blocks] permissions nlink owner group size mtime name
// R3.2: symlinks display as "name -> target" in long format.
// R3.3: entry name wrapped in ANSI color when active.
// R3.5: --time selects which timestamp to display.
// R3.6: --time-style controls timestamp formatting.
func printLongLine(e entry, cfg lsConfig, lw longWidths, pw prefixWidths) {
	var prefix string
	if cfg.showInode {
		prefix += format.PadLeft(inodeString(e), pw.inode) + " "
	}
	if cfg.showBlocks {
		prefix += format.PadLeft(blockString(e, cfg.humanReadable), pw.blocks) + " "
	}

	perm := permissionString(e.info.Mode)
	nlink := format.PadLeft(strconv.FormatUint(e.info.Nlink, 10), lw.nlink)
	owner := format.PadRight(ownerString(e.info.Uid, cfg.numericIDs), lw.owner)
	group := format.PadRight(groupString(e.info.Gid, cfg.numericIDs), lw.group)
	size := format.PadLeft(sizeString(e.info.Size, cfg.humanReadable), lw.size)
	ts := entryTime(e.info, cfg.timeSelect)
	mtime := formatEntryTime(ts, cfg.timeStyle)
	display := coloredSymlinkDisplay(e) + typeIndicator(e.info, cfg.indicator)

	fmt.Printf("%s%s %s %s %s %s %s %s\n",
		prefix, perm, nlink, owner, group, size, mtime, display)
}

// sizeString formats a file size, optionally human-readable.
// R3.5: -h uses pkg/format.HumanSize with Binary=true (1024-base).
func sizeString(size int64, humanReadable bool) string {
	if humanReadable {
		return format.HumanSize(size, format.HumanSizeOpts{Binary: true})
	}
	return strconv.FormatInt(size, 10)
}

// coloredSymlinkDisplay returns the display name with color and symlink target.
// R3.2: symlinks show "name -> target" in long format.
// R3.3: name is wrapped in ANSI color codes when color is active.
func coloredSymlinkDisplay(e entry) string {
	name := coloredEntryName(e.name, e.info)
	if e.info != nil && e.info.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(e.path)
		if err == nil {
			return name + " -> " + target
		}
	}
	return name
}

// formatEntryTime formats a timestamp for long-format display.
// R3.6: --time-style controls the format; default uses locale-like format.
func formatEntryTime(t time.Time, style string) string {
	switch style {
	case "full-iso":
		return t.Format("2006-01-02 15:04:05.000000000 -0700")
	case "long-iso":
		return t.Format("2006-01-02 15:04")
	case "iso":
		return formatISOTime(t)
	default:
		return formatDefaultTime(t)
	}
}

// formatISOTime formats time in ISO style: recent "01-02 15:04", old "2006-01-02 ".
func formatISOTime(t time.Time) string {
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02 ")
}

// formatDefaultTime formats time in locale-like style.
// R1.9: recent files show "Jan _2 15:04", older show "Jan _2  2006".
func formatDefaultTime(t time.Time) string {
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// permissionString produces the 10-character permission string.
// R1.6: file type char + owner rwx + group rwx + other rwx.
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

// ownerString returns the owner field string: numeric when -n is active,
// otherwise resolved via lookupUser.
// R4.6: -n displays numeric UID.
func ownerString(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return lookupUser(uid)
}

// groupString returns the group field string: numeric when -n is active,
// otherwise resolved via lookupGroup.
// R4.6: -n displays numeric GID.
func groupString(gid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return lookupGroup(gid)
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

// printCommaEntries prints entries in comma-separated format, wrapping at
// terminal width. R2.11: -m fills width with comma-separated entries.
func printCommaEntries(names []string) {
	if len(names) == 0 {
		return
	}
	width := termWidthOrDefault()
	col := 0
	for i, name := range names {
		nameLen := utf8.RuneCountInString(name)
		if i == 0 {
			fmt.Print(name)
			col = nameLen
			continue
		}
		needed := 2 + nameLen // ", " + name
		if col+needed > width {
			fmt.Print(",\n")
			fmt.Print(name)
			col = nameLen
		} else {
			fmt.Print(", " + name)
			col += needed
		}
	}
	fmt.Println()
}

// printHorizontalColumnar prints entries in row-major multi-column format.
// R1.13: -x sorts horizontally (across columns, then down to next row).
func printHorizontalColumnar(names []string) {
	width := termWidthOrDefault()
	numCols := computeHorizontalCols(names, width)
	if numCols <= 1 {
		for _, name := range names {
			fmt.Println(name)
		}
		return
	}
	colWidths := horizontalColWidths(names, numCols)
	for i := 0; i < len(names); i += numCols {
		end := i + numCols
		if end > len(names) {
			end = len(names)
		}
		printColumnarRow(names[i:end], colWidths)
	}
}

// computeHorizontalCols finds the maximum column count for horizontal layout.
func computeHorizontalCols(names []string, width int) int {
	maxCols := width / 2
	if maxCols < 1 {
		maxCols = 1
	}
	if maxCols > len(names) {
		maxCols = len(names)
	}
	for cols := maxCols; cols > 1; cols-- {
		cw := horizontalColWidths(names, cols)
		if horizontalTotalWidth(cw) <= width {
			return cols
		}
	}
	return 1
}

// horizontalColWidths computes per-column max widths for row-major layout.
func horizontalColWidths(names []string, numCols int) []int {
	widths := make([]int, numCols)
	for i, name := range names {
		col := i % numCols
		w := utf8.RuneCountInString(name)
		if w > widths[col] {
			widths[col] = w
		}
	}
	return widths
}

// horizontalTotalWidth returns total width needed for the given column widths.
func horizontalTotalWidth(colWidths []int) int {
	total := 0
	for i, w := range colWidths {
		total += w
		if i < len(colWidths)-1 {
			total += 2 // gap between columns
		}
	}
	return total
}

// updateWidth sets *current to v if v is larger.
func updateWidth(current *int, v int) {
	if v > *current {
		*current = v
	}
}
