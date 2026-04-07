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
	"sync"

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

// PadRight pads s on the right with spaces to the given width.
func PadRight(s string, width int) string {
	return ""
}

// PadLeft pads s on the left with spaces to the given width.
func PadLeft(s string, width int) string {
	return ""
}

// Columns arranges entries into columns that fit within termWidth.
func Columns(entries []string, termWidth int) [][]string {
	return nil
}
