// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, column alignment, and
// string padding.
//
// Implements: srd003-format
package format

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI SGR escape codes matching GNU ls LS_COLORS defaults.
// R2.4: directory=34, symlink=36, executable=32, block=33;1,
// char=33;1, socket=35, pipe=33, regular=0.
const (
	ansiReset   = "\033[0m"
	ansiDir     = "\033[01;34m" // bold blue
	ansiSymlink = "\033[01;36m" // bold cyan
	ansiExec    = "\033[01;32m" // bold green
	ansiBlock   = "\033[33;1m"  // yellow bold
	ansiChar    = "\033[33;1m"  // yellow bold
	ansiSocket  = "\033[35m"    // magenta
	ansiPipe    = "\033[33m"    // yellow
	ansiSetuid  = "\033[37;41m" // white on red
	ansiSetgid  = "\033[30;43m" // black on yellow
	ansiSticky  = "\033[37;44m" // white on blue
	ansiOWrit   = "\033[30;42m" // black on green (other-writable)
)

// colorMu protects colorOverride.
var colorMu sync.RWMutex

// colorOverride is nil when no override is set (auto-detect mode).
// When non-nil, its value forces color on or off.
var colorOverride *bool

// HumanSizeOpts configures human-readable size formatting.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string with unit suffixes.
// R3.1: Binary=true uses base 1024 with K/M/G/T/P/E suffixes.
// R3.1: Binary=false uses base 1000 with kB/MB/GB/TB suffixes.
// R3.4: Returns "0" for zero byte count.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}
	base, suffixes := siParams()
	if opts.Binary {
		base, suffixes = binaryParams()
	}
	return formatHumanSize(bytes, base, suffixes)
}

func binaryParams() (float64, []string) {
	return 1024, []string{"", "K", "M", "G", "T", "P", "E"}
}

func siParams() (float64, []string) {
	return 1000, []string{"", "kB", "MB", "GB", "TB"}
}

// formatHumanSize selects the largest unit where value >= 1.0 and formats
// with GNU coreutils precision: one decimal for values < 10, no decimal otherwise.
// R3.3: At most one decimal place when value is not integer at chosen unit.
func formatHumanSize(bytes int64, base float64, suffixes []string) string {
	neg := bytes < 0
	val := math.Abs(float64(bytes))
	idx := 0
	for idx+1 < len(suffixes) && val >= base {
		val /= base
		idx++
	}
	prefix := ""
	if neg {
		prefix = "-"
	}
	if suffixes[idx] == "" {
		return fmt.Sprintf("%s%d", prefix, abs64(bytes))
	}
	// R3.4/R3.3: GNU format — one decimal when < 10, no decimal when >= 10.
	if val < 10.0 {
		return fmt.Sprintf("%s%.1f%s", prefix, val, suffixes[idx])
	}
	return fmt.Sprintf("%s%.0f%s", prefix, val, suffixes[idx])
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// colorActive returns whether color output is currently active,
// considering the package-level override.
func colorActive() bool {
	colorMu.RLock()
	defer colorMu.RUnlock()
	if colorOverride != nil {
		return *colorOverride
	}
	return false
}

// FileTypeColor returns the ANSI color escape sequence for the given file mode.
// R2.1: Returns the escape sequence matching the file type.
// R2.3: Returns empty string when color is not enabled.
// R2.4: Uses GNU ls LS_COLORS default color assignments.
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}
	return fileTypeCode(mode)
}

// fileTypeCode returns the ANSI code for the file type, checking special
// permission bits (setuid, setgid, sticky, other-writable) before type.
func fileTypeCode(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0 && mode&os.ModeSticky != 0:
		return ansiSticky
	case mode&os.ModeDir != 0 && mode&0o002 != 0:
		return ansiOWrit
	case mode&os.ModeDir != 0:
		return ansiDir
	case mode&os.ModeSymlink != 0:
		return ansiSymlink
	case mode&os.ModeSetuid != 0:
		return ansiSetuid
	case mode&os.ModeSetgid != 0:
		return ansiSetgid
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return ansiChar
	case mode&os.ModeDevice != 0:
		return ansiBlock
	case mode&os.ModeSocket != 0:
		return ansiSocket
	case mode&os.ModeNamedPipe != 0:
		return ansiPipe
	case mode&0o111 != 0:
		return ansiExec
	default:
		return ansiReset
	}
}

// Reset returns the ANSI reset escape sequence.
// R2.2: Returns "\033[0m".
// R2.3: Returns empty string when color is not enabled.
func Reset() string {
	if !colorActive() {
		return ""
	}
	return ansiReset
}

// ColorEnabled reports whether color output is enabled for the given writer.
// R2.3: Attempts type assertion to *os.File; if successful, calls
// sys.IsTerminal on the file descriptor. Returns false for non-terminal writers.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides the automatic color detection with the given value.
// R2.6: When true, FileTypeColor and Reset return ANSI sequences regardless of TTY.
// When false, they return empty strings regardless.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any manual color override, restoring automatic detection.
// R2.7: Reverts to auto-detect mode where color is off unless explicitly enabled.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// PadRight pads s on the right with spaces to reach the target visible width.
// R1.2: Right-aligned field padding within a fixed-width column.
// R2.6/D3: Uses visible width (ANSI escapes excluded) for correct column alignment.
func PadRight(s string, width int) string {
	vw := visibleWidth(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

// PadLeft pads s on the left with spaces to reach the target visible width.
// R1.2: Left-aligned field padding within a fixed-width column.
func PadLeft(s string, width int) string {
	vw := visibleWidth(s)
	if vw >= width {
		return s
	}
	return strings.Repeat(" ", width-vw) + s
}

// visibleWidth returns the display width of s, excluding ANSI CSI escape
// sequences (ESC[ ... final_byte). R1.3: uses rune count for display width.
func visibleWidth(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

// stripANSI removes all ANSI CSI sequences (ESC[ params final_byte) from s.
// D2: Strips sequences matching ESC[ followed by parameter bytes (0x30-0x3F),
// intermediate bytes (0x20-0x2F), and a final byte (0x40-0x7E).
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i = skipCSI(s, i+2)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// skipCSI advances past a CSI sequence starting at position i (after ESC[).
// Returns the index of the first byte after the final byte of the sequence.
func skipCSI(s string, i int) int {
	for i < len(s) {
		c := s[i]
		i++
		// Final byte of a CSI sequence is in range 0x40-0x7E.
		if c >= 0x40 && c <= 0x7E {
			return i
		}
	}
	return i
}

// Columns arranges entries into rows for a column-down layout fitting within
// termWidth. R1.1: Uses the maximum number of columns that fit. R1.4: Each
// column's width is the longest entry in that column, not the global maximum.
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}
	widths := precomputeWidths(entries)
	bestCols := findMaxColumns(widths, n, termWidth)
	return buildColumnRows(entries, bestCols, n)
}

// precomputeWidths returns the visible width of each entry.
// R3.3: Strips ANSI escapes before measuring via visibleWidth.
func precomputeWidths(entries []string) []int {
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = visibleWidth(e)
	}
	return widths
}

// findMaxColumns tries column counts from n down to 1, returning the maximum
// that fits within termWidth. Uses 2-space gaps between columns.
func findMaxColumns(widths []int, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		if columnsTotalWidth(widths, numCols, numRows, n) <= termWidth {
			return numCols
		}
	}
	return 1
}

// columnsTotalWidth computes the total display width for a column layout.
// Each column's width is the max visible width of its entries plus a 2-space
// gap between columns.
func columnsTotalWidth(widths []int, numCols, numRows, n int) int {
	total := 0
	for col := range numCols {
		maxW := 0
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			if widths[idx] > maxW {
				maxW = widths[idx]
			}
		}
		total += maxW
		if col < numCols-1 {
			total += 2
		}
	}
	return total
}

// buildColumnRows arranges entries into rows for column-down (top-to-bottom)
// layout matching GNU ls default behavior.
func buildColumnRows(entries []string, numCols, n int) [][]string {
	numRows := (n + numCols - 1) / numCols
	rows := make([][]string, numRows)
	for row := range numRows {
		var r []string
		for col := range numCols {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			r = append(r, entries[idx])
		}
		rows[row] = r
	}
	return rows
}
