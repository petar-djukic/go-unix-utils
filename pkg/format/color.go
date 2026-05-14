// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	ansiReset    = "\033[0m"
	ansiDir      = "\033[01;34m"
	ansiSymlink  = "\033[01;36m"
	ansiExec     = "\033[01;32m"
	ansiFifo     = "\033[40;33m"
	ansiSocket   = "\033[01;35m"
	ansiBlockDev = "\033[01;33m"
	ansiCharDev  = "\033[01;33m"
)

var (
	colorMu       sync.RWMutex
	colorOverride *bool
)

func colorDisabled() bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()
	return override != nil && !*override
}

// FileTypeColor returns the ANSI escape sequence for a file's type.
// R2.1, R2.4, R2.6: returns empty string when color is explicitly disabled via SetColorEnabled(false).
func FileTypeColor(mode os.FileMode) string {
	if colorDisabled() {
		return ""
	}
	switch {
	case mode&os.ModeDir != 0:
		return ansiDir
	case mode&os.ModeSymlink != 0:
		return ansiSymlink
	case mode&os.ModeNamedPipe != 0:
		return ansiFifo
	case mode&os.ModeSocket != 0:
		return ansiSocket
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return ansiCharDev
	case mode&os.ModeDevice != 0:
		return ansiBlockDev
	case mode&0o111 != 0:
		return ansiExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset sequence.
// R2.2, R2.6: returns empty string when color is explicitly disabled via SetColorEnabled(false).
func Reset() string {
	if colorDisabled() {
		return ""
	}
	return ansiReset
}

// ColorEnabled reports whether color output is enabled for the given writer.
// R2.3: when no override is set, detects whether the writer is a terminal.
func ColorEnabled(w io.Writer) bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()
	if override != nil {
		return *override
	}
	type fder interface {
		Fd() uintptr
	}
	if f, ok := w.(fder); ok {
		return sys.IsTerminal(f.Fd())
	}
	return false
}

// SetColorEnabled forces color output on or off regardless of terminal detection.
// R2.5: overrides the terminal check used by ColorEnabled and FileTypeColor.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	colorOverride = &enabled
	colorMu.Unlock()
}

// ResetColorEnabled clears any color override, restoring terminal-based detection.
// R2.5: undoes a prior SetColorEnabled call.
func ResetColorEnabled() {
	colorMu.Lock()
	colorOverride = nil
	colorMu.Unlock()
}
