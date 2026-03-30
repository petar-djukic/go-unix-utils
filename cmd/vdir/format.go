// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// format.go implements output formatting for cmd/vdir (prd108-vdir R1.1-R1.6):
// sorting, long-format, multi-column, single-column, and across output
// with C-style escaping, color, classification, and alignment.
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
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sixMonths is the threshold for recent vs old mtime display in long format.
const sixMonths = 6 * 30 * 24 * time.Hour

// --- Sorting ---

// sortEntries sorts entries according to the configured sort mode.
// R1.4: C locale sort by default.
func sortEntries(entries []vdirEntry, cfg *vdirConfig) {
	if cfg.sortBy == sortNone {
		if cfg.reverse {
			reverseEntries(entries)
		}
		return
	}
	less := sortLessFunc(entries, cfg.sortBy)
	if cfg.reverse {
		orig := less
		less = func(i, j int) bool { return orig(j, i) }
	}
	sort.SliceStable(entries, less)
}

// sortLessFunc returns the comparison function for the given sort mode.
func sortLessFunc(entries []vdirEntry, sm sortMode) func(int, int) bool {
	switch sm {
	case sortTime:
		return func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.info != nil && b.info != nil {
				if !a.info.ModTime.Equal(b.info.ModTime) {
					return a.info.ModTime.After(b.info.ModTime)
				}
			}
			return a.name < b.name
		}
	case sortSize:
		return func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.info != nil && b.info != nil {
				if a.info.Size != b.info.Size {
					return a.info.Size > b.info.Size
				}
			}
			return a.name < b.name
		}
	case sortVersion:
		return func(i, j int) bool {
			return strverscmp(entries[i].name, entries[j].name) < 0
		}
	default:
		return func(i, j int) bool {
			return entries[i].name < entries[j].name
		}
	}
}

// reverseEntries reverses the slice in place.
func reverseEntries(entries []vdirEntry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}

// strverscmp compares two strings with natural version ordering.
func strverscmp(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigit(ca) && isDigit(cb) {
			r := compareDigitSeq(a[ai:], b[bi:])
			if r != 0 {
				return r
			}
			ai += digitRunLen(a[ai:])
			bi += digitRunLen(b[bi:])
			continue
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		ai++
		bi++
	}
	return len(a) - len(b)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// compareDigitSeq compares two digit sequences using strverscmp rules.
func compareDigitSeq(a, b string) int {
	if a[0] == '0' || b[0] == '0' {
		return compareFractional(a, b)
	}
	return compareIntegral(a, b)
}

// compareFractional compares digit sequences with leading zeros.
func compareFractional(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && isDigit(a[i]) && isDigit(b[i]) {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
		i++
	}
	if i < len(a) && isDigit(a[i]) {
		return 1
	}
	if i < len(b) && isDigit(b[i]) {
		return -1
	}
	return 0
}

// compareIntegral compares digit sequences as integers.
func compareIntegral(a, b string) int {
	aLen, bLen := digitRunLen(a), digitRunLen(b)
	if aLen != bLen {
		return aLen - bLen
	}
	for i := 0; i < aLen; i++ {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

func digitRunLen(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// --- Output dispatch ---

// entryPrefixWidths holds max widths for inode and block count prefixes.
type entryPrefixWidths struct {
	inode  int
	blocks int
}

// formatOutput dispatches to the appropriate output formatter.
func formatOutput(cfg *vdirConfig, entries []vdirEntry) {
	if len(entries) == 0 {
		return
	}
	pw := computePrefixWidths(cfg, entries)
	switch cfg.format {
	case formatSingle:
		formatSingleColumn(cfg, entries, pw)
	case formatLong:
		formatLongListing(cfg, entries)
	case formatAcross:
		formatAcrossColumns(cfg, entries, pw)
	case formatComma:
		formatCommaList(cfg, entries)
	default:
		formatMultiColumn(cfg, entries, pw)
	}
}

// computePrefixWidths computes max widths for inode and block prefixes.
func computePrefixWidths(cfg *vdirConfig, entries []vdirEntry) entryPrefixWidths {
	var pw entryPrefixWidths
	if !cfg.showInode && !cfg.showBlocks {
		return pw
	}
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		if cfg.showInode {
			updateMaxInt(&pw.inode, uintWidth(e.info.Ino))
		}
		if cfg.showBlocks {
			s := formatBlockCount(e.info.Blocks, cfg.humanSize)
			updateMaxInt(&pw.blocks, len(s))
		}
	}
	return pw
}

// formatEntryPrefix builds the inode and/or blocks prefix string.
func formatEntryPrefix(cfg *vdirConfig, e vdirEntry, pw entryPrefixWidths) string {
	if !cfg.showInode && !cfg.showBlocks {
		return ""
	}
	var sb strings.Builder
	if cfg.showInode && e.info != nil {
		fmt.Fprintf(&sb, "%*d ", pw.inode, e.info.Ino)
	}
	if cfg.showBlocks && e.info != nil {
		bStr := formatBlockCount(e.info.Blocks, cfg.humanSize)
		fmt.Fprintf(&sb, "%*s ", pw.blocks, bStr)
	}
	return sb.String()
}

// entryNames extracts display names from entries with optional prefix.
func entryNames(cfg *vdirConfig, entries []vdirEntry, pw entryPrefixWidths) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = formatEntryPrefix(cfg, e, pw) + entryDisplayName(cfg, e)
	}
	return names
}

// --- Single-column format ---

// formatSingleColumn prints one entry per line with C-style escaping.
func formatSingleColumn(cfg *vdirConfig, entries []vdirEntry, pw entryPrefixWidths) {
	for _, e := range entries {
		fmt.Printf("%s%s\n",
			formatEntryPrefix(cfg, e, pw),
			entryDisplayName(cfg, e))
	}
}

// --- Multi-column format (vertical fill) ---

// formatMultiColumn prints entries in vertical multi-column layout.
func formatMultiColumn(cfg *vdirConfig, entries []vdirEntry, pw entryPrefixWidths) {
	names := entryNames(cfg, entries, pw)
	rows := format.Columns(names, cfg.termWidth)
	if len(rows) == 0 {
		return
	}
	colWidths := computeGridWidths(rows)
	printColumnRows(rows, colWidths)
}

// computeGridWidths returns the maximum display width per column.
func computeGridWidths(rows [][]string) []int {
	numCols := len(rows[0])
	widths := make([]int, numCols)
	for _, row := range rows {
		for c, cell := range row {
			w := utf8.RuneCountInString(cell)
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	return widths
}

// printColumnRows prints rows with column-aligned entries using
// tab-based indentation matching GNU coreutils output.
func printColumnRows(rows [][]string, colWidths []int) {
	colStarts := computeColStarts(colWidths)
	for _, row := range rows {
		var sb strings.Builder
		curPos := 0
		for c, cell := range row {
			sb.WriteString(cell)
			curPos += utf8.RuneCountInString(cell)
			if c < len(row)-1 {
				target := colStarts[c+1]
				curPos = indentTo(&sb, curPos, target)
			}
		}
		fmt.Println(sb.String())
	}
}

// computeColStarts returns the display-column start for each column.
func computeColStarts(colWidths []int) []int {
	starts := make([]int, len(colWidths))
	pos := 0
	for c, w := range colWidths {
		starts[c] = pos
		pos += w + 2 // 2-character gap between columns
	}
	return starts
}

// indentTo advances output from 'from' to 'to' using tabs and spaces,
// matching GNU coreutils indent().
func indentTo(sb *strings.Builder, from, to int) int {
	const tabSize = 8
	for from < to {
		if to/tabSize > (from+1)/tabSize {
			sb.WriteByte('\t')
			from += tabSize - from%tabSize
		} else {
			sb.WriteByte(' ')
			from++
		}
	}
	return from
}

// --- Across-column format (horizontal fill) ---

// formatAcrossColumns prints entries in horizontal multi-column layout.
func formatAcrossColumns(cfg *vdirConfig, entries []vdirEntry, pw entryPrefixWidths) {
	names := entryNames(cfg, entries, pw)
	numCols := fitAcrossColumns(names, cfg.termWidth)
	rows := chunkNames(names, numCols)
	if len(rows) == 0 {
		return
	}
	colWidths := computeGridWidths(rows)
	printColumnRows(rows, colWidths)
}

// fitAcrossColumns returns the max columns that fit within termWidth.
func fitAcrossColumns(names []string, termWidth int) int {
	for numCols := len(names); numCols > 1; numCols-- {
		if acrossGridFits(names, numCols, termWidth) {
			return numCols
		}
	}
	return 1
}

// acrossGridFits checks whether names in numCols fit within termWidth.
func acrossGridFits(names []string, numCols, termWidth int) bool {
	colWidths := make([]int, numCols)
	for i, name := range names {
		col := i % numCols
		w := utf8.RuneCountInString(name)
		if w > colWidths[col] {
			colWidths[col] = w
		}
	}
	total := 0
	for i, w := range colWidths {
		total += w
		if i < numCols-1 {
			total += 2
		}
	}
	return total <= termWidth
}

// chunkNames splits names into rows of numCols entries each.
func chunkNames(names []string, numCols int) [][]string {
	if len(names) == 0 {
		return nil
	}
	rows := make([][]string, 0, (len(names)+numCols-1)/numCols)
	for i := 0; i < len(names); i += numCols {
		end := i + numCols
		if end > len(names) {
			end = len(names)
		}
		rows = append(rows, names[i:end])
	}
	return rows
}

// --- Comma-separated format (-m) ---

// formatCommaList prints entries in comma-separated format, wrapping at
// the terminal width. R1.5: -m overrides the default long format.
func formatCommaList(cfg *vdirConfig, entries []vdirEntry) {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = entryDisplayName(cfg, e)
	}
	printCommaWrapped(names, cfg.termWidth)
}

// printCommaWrapped prints names separated by ", " with line wrapping.
func printCommaWrapped(names []string, termWidth int) {
	if len(names) == 0 {
		return
	}
	col := 0
	for i, name := range names {
		suffix := ""
		if i < len(names)-1 {
			suffix = ","
		}
		entry := name + suffix
		entryLen := utf8.RuneCountInString(entry)
		if i > 0 {
			if col+1+entryLen > termWidth {
				fmt.Println()
				col = 0
			} else {
				fmt.Print(" ")
				col++
			}
		}
		fmt.Print(entry)
		col += entryLen
	}
	fmt.Println()
}

// --- Long format ---

// columnWidths holds computed widths for aligned long-format columns.
type columnWidths struct {
	nlink  int
	owner  int
	group  int
	size   int
	inode  int
	blocks int
}

// formatLongListing prints long-format output with aligned columns.
func formatLongListing(cfg *vdirConfig, entries []vdirEntry) {
	cw := computeColumnWidths(cfg, entries)
	for _, e := range entries {
		printLongEntry(cfg, e, cw)
	}
}

// computeColumnWidths calculates alignment widths for long format.
func computeColumnWidths(cfg *vdirConfig, entries []vdirEntry) columnWidths {
	var cw columnWidths
	for _, e := range entries {
		if e.info == nil {
			continue
		}
		updateMaxInt(&cw.nlink, uintWidth(e.info.Nlink))
		updateMaxInt(&cw.owner, len(resolveOwner(e.info.Uid, cfg.numericIDs)))
		updateMaxInt(&cw.group, len(resolveGroup(e.info.Gid, cfg.numericIDs)))
		updateMaxInt(&cw.size, len(formatSize(e.info.Size, cfg.humanSize)))
		if cfg.showInode {
			updateMaxInt(&cw.inode, uintWidth(e.info.Ino))
		}
		if cfg.showBlocks {
			s := formatBlockCount(e.info.Blocks, cfg.humanSize)
			updateMaxInt(&cw.blocks, len(s))
		}
	}
	return cw
}

// printLongEntry prints a single entry in long format.
func printLongEntry(cfg *vdirConfig, e vdirEntry, cw columnWidths) {
	if e.info == nil {
		fmt.Println(escapeName(e.name))
		return
	}
	fi := e.info
	prefix := longEntryPrefix(cfg, fi, cw)
	perm := permissionString(fi.Mode)
	owner := resolveOwner(fi.Uid, cfg.numericIDs)
	group := resolveGroup(fi.Gid, cfg.numericIDs)
	mtime := formatTime(fi.ModTime)
	name := entryDisplayName(cfg, e)
	if e.link != "" {
		name = coloredEscapedName(e) + " -> " + escapeName(e.link)
	}
	sizeStr := formatSize(fi.Size, cfg.humanSize)
	fmt.Printf("%s%s %*d %-*s %-*s %*s %s %s\n",
		prefix, perm,
		cw.nlink, fi.Nlink,
		cw.owner, owner,
		cw.group, group,
		cw.size, sizeStr,
		mtime, name,
	)
}

// longEntryPrefix builds the inode/blocks prefix for long format.
func longEntryPrefix(cfg *vdirConfig, fi *sys.FileInfo, cw columnWidths) string {
	if !cfg.showInode && !cfg.showBlocks {
		return ""
	}
	var sb strings.Builder
	if cfg.showInode {
		fmt.Fprintf(&sb, "%*d ", cw.inode, fi.Ino)
	}
	if cfg.showBlocks {
		bStr := formatBlockCount(fi.Blocks, cfg.humanSize)
		fmt.Fprintf(&sb, "%*s ", cw.blocks, bStr)
	}
	return sb.String()
}

// printTotalLine prints the "total N" block count line.
// R1.3: print total block count before directory listing.
func printTotalLine(cfg *vdirConfig, entries []vdirEntry) {
	var totalBlocks int64
	for _, e := range entries {
		if e.info != nil {
			totalBlocks += e.info.Blocks
		}
	}
	kb := totalBlocks / 2
	if cfg.humanSize {
		fmt.Printf("total %s\n", gnuHumanSize(kb*1024))
	} else {
		fmt.Printf("total %d\n", kb)
	}
}

// --- Permission string ---

// permissionString builds the 10-character permission string.
func permissionString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	perm := mode.Perm()
	buf[1] = permBit(perm&0o400 != 0, 'r')
	buf[2] = permBit(perm&0o200 != 0, 'w')
	buf[3] = execSpecial(perm&0o100 != 0, mode&os.ModeSetuid != 0, 's', 'S')
	buf[4] = permBit(perm&0o040 != 0, 'r')
	buf[5] = permBit(perm&0o020 != 0, 'w')
	buf[6] = execSpecial(perm&0o010 != 0, mode&os.ModeSetgid != 0, 's', 'S')
	buf[7] = permBit(perm&0o004 != 0, 'r')
	buf[8] = permBit(perm&0o002 != 0, 'w')
	buf[9] = execSpecial(perm&0o001 != 0, mode&os.ModeSticky != 0, 't', 'T')
	return string(buf[:])
}

// fileTypeChar returns the leading character for the permission string.
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

func permBit(set bool, ch byte) byte {
	if set {
		return ch
	}
	return '-'
}

// execSpecial returns the execute character accounting for special bits.
func execSpecial(exec, special bool, lower, upper byte) byte {
	if special {
		if exec {
			return lower
		}
		return upper
	}
	if exec {
		return 'x'
	}
	return '-'
}

// --- User/group/time/size ---

// resolveOwner looks up the username for a UID.
func resolveOwner(uid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// resolveGroup looks up the group name for a GID.
func resolveGroup(gid uint32, numeric bool) string {
	if numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// formatTime formats the modification time for long format.
func formatTime(t time.Time) string {
	if time.Since(t) < sixMonths {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// formatSize formats a file size, optionally human-readable.
func formatSize(size int64, human bool) string {
	if human {
		return gnuHumanSize(size)
	}
	return strconv.FormatInt(size, 10)
}

// formatBlockCount formats blocks in 1K-block units.
func formatBlockCount(blocks int64, human bool) string {
	kb := blocks / 2
	if human {
		return gnuHumanSize(kb * 1024)
	}
	return strconv.FormatInt(kb, 10)
}

// gnuHumanSuffixes are 1024-base unit suffixes matching GNU ls -h.
var gnuHumanSuffixes = []string{"K", "M", "G", "T", "P", "E"}

// gnuHumanSize formats bytes matching GNU ls -h conventions.
func gnuHumanSize(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10)
	}
	val := float64(bytes)
	for i, suffix := range gnuHumanSuffixes {
		val /= 1024.0
		if i == len(gnuHumanSuffixes)-1 || val < 1024.0 {
			return formatGNUValue(val, suffix)
		}
	}
	return strconv.FormatInt(bytes, 10)
}

// formatGNUValue renders a scaled value with GNU-style precision.
func formatGNUValue(val float64, suffix string) string {
	if val >= 10 {
		return fmt.Sprintf("%d%s", int64(math.Ceil(val)), suffix)
	}
	rounded := math.Ceil(val*10) / 10
	return fmt.Sprintf("%.1f%s", rounded, suffix)
}

// --- Display name and escaping ---

// entryDisplayName builds the display name with escaping, color, and indicator.
// R1.2: C-style escaping always applied.
func entryDisplayName(cfg *vdirConfig, e vdirEntry) string {
	escaped := escapeName(e.name)
	name := escaped
	if e.info != nil {
		c := format.FileTypeColor(e.info.Mode)
		if c != "" {
			name = c + escaped + format.Reset()
		}
	}
	if cfg.classify && e.info != nil {
		name += classifyIndicator(e.info.Mode)
	}
	return name
}

// coloredEscapedName returns the escaped name with optional color, no indicator.
func coloredEscapedName(e vdirEntry) string {
	escaped := escapeName(e.name)
	if e.info != nil {
		c := format.FileTypeColor(e.info.Mode)
		if c != "" {
			return c + escaped + format.Reset()
		}
	}
	return escaped
}

// classifyIndicator returns the -F indicator for a file mode.
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

// escapeName applies C-style backslash escaping to a filename.
// R1.2: escape non-printable characters, backslashes, and spaces.
func escapeName(name string) string {
	var sb strings.Builder
	sb.Grow(len(name))
	for i := 0; i < len(name); i++ {
		b := name[i]
		if esc, ok := escapeChar(b); ok {
			sb.WriteString(esc)
		} else if b < 0x20 || b >= 0x7f {
			fmt.Fprintf(&sb, "\\%03o", b)
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

// escapeChar returns the C-style escape for named control characters.
func escapeChar(b byte) (string, bool) {
	switch b {
	case '\\':
		return "\\\\", true
	case '\a':
		return "\\a", true
	case '\b':
		return "\\b", true
	case '\f':
		return "\\f", true
	case '\n':
		return "\\n", true
	case '\r':
		return "\\r", true
	case '\t':
		return "\\t", true
	case '\v':
		return "\\v", true
	case ' ':
		return "\\ ", true
	default:
		return "", false
	}
}

// --- Utility ---

func updateMaxInt(cur *int, v int) {
	if v > *cur {
		*cur = v
	}
}

func uintWidth(n uint64) int {
	return len(strconv.FormatUint(n, 10))
}
