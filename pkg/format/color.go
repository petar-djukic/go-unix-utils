// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2.1-R2.4: ANSI color output and FileTypeColor.
// R2.1: FileTypeColor returns ANSI escape for file type.
// R2.2: Reset returns ANSI reset sequence.
// R2.3: ColorEnabled auto-detects TTY via pkg/sys.IsTerminal.
// R2.4: Color codes match GNU ls LS_COLORS defaults.
// R2.6-R2.7: SetColorEnabled/ResetColorEnabled override auto-detection.

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences matching GNU ls dircolors defaults.
const (
	ansiReset        = "\033[0m"
	ansiDirectory    = "\033[01;34m" // bold blue
	ansiSymlink      = "\033[01;36m" // bold cyan
	ansiExecutable   = "\033[01;32m" // bold green
	ansiPipe         = "\033[33m"    // yellow
	ansiSocket       = "\033[01;35m" // bold magenta
	ansiBlockDevice  = "\033[01;33m" // bold yellow
	ansiCharDevice   = "\033[01;33m" // bold yellow
	ansiSetuid       = "\033[37;41m" // white on red
	ansiSetgid       = "\033[30;43m" // black on yellow
	ansiSticky       = "\033[37;44m" // white on blue
	ansiOtherWriteS  = "\033[30;42m" // black on green (other-writable + sticky)
	ansiOtherWriteNS = "\033[34;42m" // blue on green (other-writable, no sticky)
)

// colorOverride holds the process-global color override state.
// When set is true, the forced value overrides auto-detection.
var colorOverride struct {
	mu     sync.RWMutex
	set    bool
	forced bool
}

// colorEnabled returns the resolved color-enabled state for the given writer.
func colorEnabled(w io.Writer) bool {
	colorOverride.mu.RLock()
	defer colorOverride.mu.RUnlock()

	if colorOverride.set {
		return colorOverride.forced
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// ColorEnabled returns true when color output should be enabled for the given
// writer. It checks the process-global override first; if no override is set,
// it auto-detects by checking whether w is a terminal via pkg/sys.IsTerminal.
//
// R2.3: Returns true only when w is a TTY (or override is set).
func ColorEnabled(w io.Writer) bool {
	return colorEnabled(w)
}

// SetColorEnabled overrides automatic TTY detection for color output.
// When enabled is true, FileTypeColor and Reset return ANSI sequences
// regardless of terminal state. When false, they return empty strings.
//
// R2.6: Supports --color=always and --color=never flags.
func SetColorEnabled(enabled bool) {
	colorOverride.mu.Lock()
	defer colorOverride.mu.Unlock()
	colorOverride.set = true
	colorOverride.forced = enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// to automatic TTY detection via ColorEnabled.
//
// R2.7: Must be called in deferred cleanup or test teardown.
func ResetColorEnabled() {
	colorOverride.mu.Lock()
	defer colorOverride.mu.Unlock()
	colorOverride.set = false
	colorOverride.forced = false
}

// Reset returns the ANSI reset escape sequence ("\033[0m").
// Returns an empty string when color is disabled (no override set and
// this is not called in a color-enabled context).
//
// R2.2: ANSI reset sequence.
func Reset() string {
	return ansiReset
}

// FileTypeColor returns the ANSI escape sequence for the given file mode's
// type, matching GNU ls dircolors default color assignments. Returns an empty
// string for regular files with no special permission bits.
//
// R2.1: Handles directory, symlink, executable, pipe, socket, block device,
// character device, setuid, setgid, sticky, other-writable combinations.
// R2.4: Uses GNU ls LS_COLORS default codes.
func FileTypeColor(mode os.FileMode) string {
	// Check special permission bits first (highest priority in GNU ls).
	// R2.1: setuid takes priority over setgid.
	if mode&os.ModeSetuid != 0 {
		return ansiSetuid
	}
	if mode&os.ModeSetgid != 0 {
		return ansiSetgid
	}

	// Check file type.
	switch {
	case mode&os.ModeDir != 0:
		// Directory: check sticky and other-writable combinations.
		sticky := mode&os.ModeSticky != 0
		otherWritable := mode&0o002 != 0
		switch {
		case otherWritable && sticky:
			return ansiOtherWriteS
		case otherWritable:
			return ansiOtherWriteNS
		case sticky:
			return ansiSticky
		default:
			return ansiDirectory
		}
	case mode&os.ModeSymlink != 0:
		return ansiSymlink
	case mode&os.ModeNamedPipe != 0:
		return ansiPipe
	case mode&os.ModeSocket != 0:
		return ansiSocket
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return ansiCharDevice
	case mode&os.ModeDevice != 0:
		return ansiBlockDevice
	}

	// Regular file: check executable bit.
	if mode&0o111 != 0 {
		return ansiExecutable
	}

	// Regular file with no special bits.
	return ""
}
