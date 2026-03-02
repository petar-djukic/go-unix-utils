// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-format (R2)
package format

import (
	"io"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// ANSI SGR escape sequences for file types, matching GNU ls LS_COLORS defaults.
// (prd003-format R2.4)
const (
	ansiReset    = "\033[0m"
	ansiDir      = "\033[34m"    // blue
	ansiSymlink  = "\033[36m"    // cyan
	ansiExec     = "\033[32m"    // green
	ansiBlock    = "\033[33;1m"  // yellow bold
	ansiChar     = "\033[33;1m"  // yellow bold
	ansiSocket   = "\033[35m"    // magenta
	ansiPipe     = "\033[33m"    // yellow
	ansiRegular  = "\033[0m"     // reset/default
	ansiSetuid   = "\033[37;41m" // white on red
	ansiSetgid   = "\033[30;43m" // black on yellow
	ansiSticky   = "\033[37;44m" // white on blue
	ansiOW       = "\033[34;42m" // blue on green (other-writable directory)
	ansiStickyOW = "\033[30;42m" // black on green (sticky + other-writable directory)
)

// colorMode represents the three-state color override: auto-detect, forced on,
// or forced off. (prd003-format R2.6, R2.7)
type colorMode int

const (
	colorAuto     colorMode = iota // detect via TTY check on stdout
	colorForceOn                   // --color=always
	colorForceOff                  // --color=never
)

var (
	activeColorMode = colorAuto
	colorMu         sync.Mutex
)

// FileTypeColor returns the ANSI escape sequence for the given file mode.
// Returns an empty string when color output is disabled. Checks permission-based
// overrides (setuid, setgid, sticky, other-writable) before file type.
// (prd003-format R2.1, R2.4, R2.6)
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}

	// Permission-based overrides have highest priority (per GNU ls).
	if mode&os.ModeSetuid != 0 {
		return ansiSetuid
	}
	if mode&os.ModeSetgid != 0 {
		return ansiSetgid
	}

	// Directory variants with sticky and other-writable combinations.
	if mode.IsDir() {
		if mode&os.ModeSticky != 0 && mode&0002 != 0 {
			return ansiStickyOW
		}
		if mode&0002 != 0 {
			return ansiOW
		}
		if mode&os.ModeSticky != 0 {
			return ansiSticky
		}
		return ansiDir
	}

	// File type determination.
	typ := mode.Type()
	switch {
	case typ&os.ModeSymlink != 0:
		return ansiSymlink
	case typ&os.ModeNamedPipe != 0:
		return ansiPipe
	case typ&os.ModeSocket != 0:
		return ansiSocket
	case typ&os.ModeDevice != 0 && typ&os.ModeCharDevice != 0:
		return ansiChar
	case typ&os.ModeDevice != 0:
		return ansiBlock
	default:
		// Regular file: check executable permission bits.
		if mode&0111 != 0 {
			return ansiExec
		}
		return ansiRegular
	}
}

// Reset returns the ANSI SGR reset sequence ("\033[0m"), or an empty string
// when color output is disabled. (prd003-format R2.2, R2.6)
func Reset() string {
	if !colorActive() {
		return ""
	}
	return ansiReset
}

// ColorEnabled reports whether color output should be enabled for the given
// writer. Returns true only when w is backed by a terminal file descriptor.
// Non-*os.File writers (pipes, buffers) always return false.
// (prd003-format R2.3)
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection. When enabled is true,
// FileTypeColor and Reset return ANSI sequences regardless of the output
// target. When enabled is false, they return empty strings regardless. The
// override is process-global. (prd003-format R2.6)
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	if enabled {
		activeColorMode = colorForceOn
	} else {
		activeColorMode = colorForceOff
	}
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled. (prd003-format R2.7)
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	activeColorMode = colorAuto
}

// colorActive returns the effective color state: forced on/off if an override
// is set, otherwise auto-detect by checking whether stdout is a terminal.
func colorActive() bool {
	colorMu.Lock()
	mode := activeColorMode
	colorMu.Unlock()
	switch mode {
	case colorForceOn:
		return true
	case colorForceOff:
		return false
	default:
		return isTerminal(os.Stdout.Fd())
	}
}

// isTerminal reports whether the file descriptor refers to a terminal, detected
// via the TIOCGWINSZ ioctl. This is a local implementation pending pkg/sys
// availability. (prd003-format R2.3)
func isTerminal(fd uintptr) bool {
	// struct winsize: 4 x uint16 = 8 bytes.
	var ws [8]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws[0])))
	return errno == 0
}
