// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd108-vdir display logic: long format output with total line,
// columnar layout, C-style filename escaping, permission strings, time
// formatting, and comparison functions. R1.1: long format is the default.
// R1.2: C-style escaping is the default quoting style.
// R1.3: "total N" block count line precedes directory listing.
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

// defaultTermWidth is used when stdout is not a TTY.
const defaultTermWidth = 80

// defaultTabSize matches GNU ls tab stop for column alignment.
const defaultTabSize = 8

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
func displayEntries(entries []entry, cfg vdirConfig) int {
	mode := resolveOutputMode(cfg)
	if mode == modeLong {
		return printLongEntries(entries, cfg)
	}
	printNonLongEntries(entries, cfg)
	return 0
}

// displayDirEntries outputs directory entries with total line when needed.
func displayDirEntries(entries []entry, cfg vdirConfig) int {
	mode := resolveOutputMode(cfg)
	if mode == modeLong {
		return printLongDir(entries, cfg)
	}
	if cfg.showBlocks {
		printTotalLine(entries, cfg)
	}
	printNonLongEntries(entries, cfg)
	return 0
}

// printTotalLine prints the "total N" line for -l or -s.
// R1.3: total = sum(Blocks)/2 in 1K-block units.
func printTotalLine(entries []entry, cfg vdirConfig) {
	var totalBlocks int64
	for _, e := range entries {
		if e.info != nil {
			totalBlocks += e.info.Blocks
		}
	}
	kBlocks := totalBlocks / 2
	if cfg.humanReadable {
		fmt.Printf("total %s\n", format.HumanSize(kBlocks*1024, format.HumanSizeOpts{Binary: true}))
	} else {
		fmt.Printf("total %d\n", kBlocks)
	}
}

// printNonLongEntries displays entries in single-column or columnar format.
func printNonLongEntries(entries []entry, cfg vdirConfig) {
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
func buildDisplayNames(entries []entry, cfg vdirConfig, pw prefixWidths) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = buildDisplayName(e, cfg, pw)
	}
	return names
}

// buildDisplayName constructs a single display string with optional prefixes.
// R1.2: entry names are escaped with C-style backslash sequences.
func buildDisplayName(e entry, cfg vdirConfig, pw prefixWidths) string {
	var parts []string
	if cfg.showInode {
		parts = append(parts, format.PadLeft(inodeString(e), pw.inode))
	}
	if cfg.showBlocks {
		parts = append(parts, format.PadLeft(blockString(e, cfg.humanReadable), pw.blocks))
	}
	name := coloredEntryName(escapeFilename(e.name), e.info)
	parts = append(parts, name+typeIndicator(e.info, cfg.indicator))
	return strings.Join(parts, " ")
}

// coloredEntryName wraps a name in ANSI color codes based on file type.
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
func computePrefixWidths(entries []entry, cfg vdirConfig) prefixWidths {
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
func inodeString(e entry) string {
	if e.info == nil {
		return "?"
	}
	return strconv.FormatUint(e.info.Ino, 10)
}

// blockString returns the block count in 1K-block units as a string.
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
// R1.1: vdir defaults to long format regardless of TTY.
func resolveOutputMode(cfg vdirConfig) outputMode {
	if cfg.output != modeDefault {
		return cfg.output
	}
	return modeLong
}

// typeIndicator returns the suffix character for -F or -p.
func typeIndicator(info *sys.FileInfo, im indicatorMode) string {
	if im == indicatorNone || info == nil {
		return ""
	}
	if info.Mode.IsDir() {
		return "/"
	}
	if im == indicatorSlash {
		return ""
	}
	return classifyIndicator(info.Mode)
}

// classifyIndicator returns the -F indicator for non-directory entries.
func classifyIndicator(mode os.FileMode) string {
	switch {
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

// printColumnar prints entries in multi-column format with tab-based alignment.
func printColumnar(names []string) {
	if len(names) == 0 {
		return
	}
	width := termWidthOrDefault()
	rows := format.Columns(names, width)
	colWidths := computeColumnWidths(rows)
	colStarts := computeColStarts(colWidths)
	for _, row := range rows {
		printTabAlignedRow(row, colStarts)
	}
}

// termWidthOrDefault returns terminal width or defaultTermWidth.
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

// computeColStarts calculates the starting position of each column.
func computeColStarts(colWidths []int) []int {
	starts := make([]int, len(colWidths))
	pos := 0
	for i, w := range colWidths {
		starts[i] = pos
		pos += w + 2 // width + minimum 2-char gap
	}
	return starts
}

// printTabAlignedRow prints a single row using tab-based alignment.
func printTabAlignedRow(row []string, colStarts []int) {
	pos := 0
	for i, e := range row {
		fmt.Print(e)
		pos += utf8.RuneCountInString(e)
		if i < len(row)-1 {
			pad := tabPad(pos, colStarts[i+1])
			fmt.Print(pad)
			pos = colStarts[i+1]
		}
	}
	fmt.Println()
}

// tabPad returns a mix of tabs and spaces to move from pos to target,
// matching GNU coreutils indent() behavior.
func tabPad(pos, target int) string {
	if pos >= target {
		return ""
	}
	var buf []byte
	for pos < target {
		if defaultTabSize != 0 && target/defaultTabSize > (pos+1)/defaultTabSize {
			buf = append(buf, '\t')
			pos += defaultTabSize - pos%defaultTabSize
		} else {
			buf = append(buf, ' ')
			pos++
		}
	}
	return string(buf)
}

// printLongDir prints directory entries in long format with total line.
// R1.3: "total N" block count line precedes directory listing.
func printLongDir(entries []entry, cfg vdirConfig) int {
	printTotalLine(entries, cfg)
	return printLongLines(entries, cfg)
}

// printLongEntries prints file-argument entries in long format (no total line).
func printLongEntries(entries []entry, cfg vdirConfig) int {
	return printLongLines(entries, cfg)
}

// printLongLines prints long-format output with aligned columns.
func printLongLines(entries []entry, cfg vdirConfig) int {
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
func computeLongWidths(entries []entry, cfg vdirConfig) longWidths {
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
// R1.2: filename is C-style escaped in long format output.
func printLongLine(e entry, cfg vdirConfig, lw longWidths, pw prefixWidths) {
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
func sizeString(size int64, humanReadable bool) string {
	if humanReadable {
		return format.HumanSize(size, format.HumanSizeOpts{Binary: true})
	}
	return strconv.FormatInt(size, 10)
}

// coloredSymlinkDisplay returns the display name with color and symlink target.
// R1.2: name is C-style escaped.
func coloredSymlinkDisplay(e entry) string {
	name := coloredEntryName(escapeFilename(e.name), e.info)
	if e.info != nil && e.info.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(e.path)
		if err == nil {
			return name + " -> " + target
		}
	}
	return name
}

// entryTime returns the selected timestamp from a FileInfo.
func entryTime(info *sys.FileInfo, tf timeField) time.Time {
	switch tf {
	case timeAccess:
		return info.AccessTime
	case timeChange:
		return info.ChangeTime
	default:
		return info.ModTime
	}
}

// formatEntryTime formats a timestamp for long-format display.
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

// formatISOTime formats time in ISO style.
func formatISOTime(t time.Time) string {
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02 ")
}

// formatDefaultTime formats time in locale-like style.
func formatDefaultTime(t time.Time) string {
	if time.Since(t) < sixMonths && time.Since(t) >= 0 {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// permissionString produces the 10-character permission string.
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

// ownerString returns the owner field string.
func ownerString(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return lookupUser(uid)
}

// groupString returns the group field string.
func groupString(gid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return lookupGroup(gid)
}

// lookupUser resolves a UID to a username.
func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// lookupGroup resolves a GID to a group name.
func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// printCommaEntries prints entries in comma-separated format.
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
		needed := 2 + nameLen
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
	colStarts := computeColStarts(colWidths)
	for i := 0; i < len(names); i += numCols {
		end := i + numCols
		if end > len(names) {
			end = len(names)
		}
		printTabAlignedRow(names[i:end], colStarts)
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

// horizontalTotalWidth returns total width needed for given column widths.
func horizontalTotalWidth(colWidths []int) int {
	total := 0
	for i, w := range colWidths {
		total += w
		if i < len(colWidths)-1 {
			total += 2
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

// escapeFilename applies C-style backslash escaping to a filename.
// R1.2: escapes backslash, newline, tab, control chars, spaces, and
// non-ASCII bytes using C-style sequences or octal notation.
func escapeFilename(name string) string {
	needsEscape := false
	for i := 0; i < len(name); i++ {
		if mustEscape(name[i]) {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return name
	}
	var buf strings.Builder
	buf.Grow(len(name) * 2)
	for i := 0; i < len(name); i++ {
		writeEscapedByte(&buf, name[i])
	}
	return buf.String()
}

// mustEscape returns true if the byte needs C-style escaping.
func mustEscape(b byte) bool {
	return b == '\\' || b == ' ' || b < 0x20 || b >= 0x7f
}

// writeEscapedByte writes a single byte in C-style escaped form.
func writeEscapedByte(buf *strings.Builder, b byte) {
	switch b {
	case '\\':
		buf.WriteString("\\\\")
	case '\a':
		buf.WriteString("\\a")
	case '\b':
		buf.WriteString("\\b")
	case '\t':
		buf.WriteString("\\t")
	case '\n':
		buf.WriteString("\\n")
	case '\v':
		buf.WriteString("\\v")
	case '\f':
		buf.WriteString("\\f")
	case '\r':
		buf.WriteString("\\r")
	case ' ':
		buf.WriteString("\\ ")
	default:
		if b < 0x20 || b >= 0x7f {
			fmt.Fprintf(buf, "\\%03o", b)
		} else {
			buf.WriteByte(b)
		}
	}
}

// compareByName sorts alphabetically by name in C locale order.
func compareByName(a, b entry) bool {
	return a.name < b.name
}

// compareByTimeField sorts by the selected time field, newest first.
func compareByTimeField(a, b entry, tf timeField) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a, b)
	}
	at := entryTime(a.info, tf)
	bt := entryTime(b.info, tf)
	if !at.Equal(bt) {
		return at.After(bt)
	}
	return a.name < b.name
}

// compareBySize sorts by file size, largest first.
func compareBySize(a, b entry) bool {
	if a.info == nil || b.info == nil {
		return compareByName(a, b)
	}
	if a.info.Size != b.info.Size {
		return a.info.Size > b.info.Size
	}
	return a.name < b.name
}

// compareByVersion sorts using version sort semantics.
func compareByVersion(a, b entry) bool {
	return versionLess(a.name, b.name)
}

// versionLess compares two strings using version sort semantics.
func versionLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigit(ca) && isDigit(cb) {
			cmp := compareDigitRuns(a, b, &ai, &bi)
			if cmp != 0 {
				return cmp < 0
			}
			continue
		}
		if ca != cb {
			return ca < cb
		}
		ai++
		bi++
	}
	return len(a) < len(b)
}

// compareDigitRuns extracts and compares digit runs numerically.
func compareDigitRuns(a, b string, ai, bi *int) int {
	na := extractDigitRun(a, ai)
	nb := extractDigitRun(b, bi)
	ta := trimLeadingZeros(na)
	tb := trimLeadingZeros(nb)
	if len(ta) != len(tb) {
		return len(ta) - len(tb)
	}
	return strings.Compare(ta, tb)
}

// extractDigitRun returns the digit substring starting at *pos.
func extractDigitRun(s string, pos *int) string {
	start := *pos
	for *pos < len(s) && isDigit(s[*pos]) {
		*pos++
	}
	return s[start:*pos]
}

// trimLeadingZeros removes leading zeros, preserving at least one digit.
func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}

// isDigit returns true if c is an ASCII digit.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
