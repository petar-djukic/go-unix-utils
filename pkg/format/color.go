// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2.1-R2.7: ANSI color output, terminal detection,
// color override control (R2.6-R2.7), and utility context for ls colorized
// output (R2.5).

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences for file type colors, matching GNU ls LS_COLORS defaults.
// R2.1, R2.4: color assignments per file type.
const (
	colorReset            = "\033[0m"
	colorBoldBlue         = "\033[01;34m" // directory
	colorBoldCyan         = "\033[01;36m" // symlink
	colorBoldGreen        = "\033[01;32m" // executable
	colorWhiteOnRed       = "\033[37;41m" // setuid
	colorBlackOnYellow    = "\033[30;43m" // setgid
	colorWhiteOnBlue      = "\033[37;44m" // sticky directory
	colorBlackOnGreen     = "\033[30;42m" // other-writable directory
	colorBoldYellowOnBlk  = "\033[01;33;40m" // block device
	colorBoldYellowOnBlk2 = "\033[01;33;40m" // char device
	colorYellow           = "\033[33m"       // named pipe (FIFO)
	colorBoldMagenta      = "\033[01;35m"    // socket
)

// colorOverride holds the process-global color override.
// D3: nil means "not set / use auto-detection"; non-nil overrides.
var (
	colorOverrideMu sync.RWMutex
	colorOverride   *bool
)

// FileTypeColor returns the ANSI escape sequence for a file's type based on its
// mode bits. Returns empty string for regular files with no special bits.
//
// R2.1: maps file types to ANSI color codes.
// D4: checks mode bits in priority order matching GNU ls LS_COLORS defaults:
// setuid > setgid > sticky > other-writable > symlink > directory > pipe >
// socket > block device > char device > executable > regular.
func FileTypeColor(mode os.FileMode) string {
	// R2.1: setuid (regardless of file type)
	if mode&os.ModeSetuid != 0 {
		return colorWhiteOnRed
	}

	// R2.1: setgid (regardless of file type)
	if mode&os.ModeSetgid != 0 {
		return colorBlackOnYellow
	}

	// R2.1: sticky directory
	if mode&os.ModeSticky != 0 && mode.IsDir() {
		return colorWhiteOnBlue
	}

	// R2.1: other-writable directory (perm bit 0o002)
	if mode.IsDir() && mode&0o002 != 0 {
		return colorBlackOnGreen
	}

	// R2.1: symlink
	if mode&os.ModeSymlink != 0 {
		return colorBoldCyan
	}

	// R2.1: directory
	if mode.IsDir() {
		return colorBoldBlue
	}

	// R2.1: named pipe (FIFO)
	if mode&os.ModeNamedPipe != 0 {
		return colorYellow
	}

	// R2.1: socket
	if mode&os.ModeSocket != 0 {
		return colorBoldMagenta
	}

	// R2.1: block device
	if mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0 {
		return colorBoldYellowOnBlk
	}

	// R2.1: char device
	if mode&os.ModeCharDevice != 0 {
		return colorBoldYellowOnBlk2
	}

	// R2.1: executable (any execute bit set on a regular file)
	if mode&0o111 != 0 {
		return colorBoldGreen
	}

	// Regular file with no special bits.
	return ""
}

// Reset returns the ANSI reset escape sequence.
//
// R2.2: returns "\033[0m" to restore terminal defaults.
func Reset() string {
	return colorReset
}

// ColorEnabled returns true when color output should be used for the given writer.
//
// R2.3: auto-detects TTY by extracting Fd() from *os.File and calling sys.IsTerminal.
// R2.4: a package-level override set via SetColorEnabled takes precedence.
func ColorEnabled(w io.Writer) bool {
	colorOverrideMu.RLock()
	override := colorOverride
	colorOverrideMu.RUnlock()

	// R2.4: override takes precedence over auto-detection.
	if override != nil {
		return *override
	}

	// R2.3: auto-detect by checking if writer is an *os.File connected to a terminal.
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides the automatic TTY detection in ColorEnabled.
// When enabled is true, ColorEnabled returns true regardless of terminal status.
// When enabled is false, ColorEnabled returns false regardless.
//
// R2.5, R2.6: process-global override for --color=always and --color=never.
// R2.5: ls suppresses color when stdout is not a TTY or --color=never is set.
func SetColorEnabled(enabled bool) {
	colorOverrideMu.Lock()
	colorOverride = &enabled
	colorOverrideMu.Unlock()
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// ColorEnabled to automatic TTY detection.
//
// R2.5, R2.7: clears the override so auto-detection resumes.
// R2.5: utilities call ResetColorEnabled in cleanup to avoid leaking overrides.
func ResetColorEnabled() {
	colorOverrideMu.Lock()
	colorOverride = nil
	colorOverrideMu.Unlock()
}
