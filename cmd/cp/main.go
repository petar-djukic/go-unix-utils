// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd056-cp R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "cp"

// cpOpts holds all flag state for a cp invocation.
type cpOpts struct {
	recursive   bool
	dereference bool // -L: follow symlinks in source
	noDerefer   bool // -P: preserve symlinks (default with -r)
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	var opts cpOpts

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--recursive":
			opts.recursive = true
		case arg == "--dereference":
			opts.dereference = true
		case arg == "--no-dereference":
			opts.noDerefer = true
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option: ignore for forward compatibility.
			operands = append(operands, arg)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options.
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'r', 'R':
					opts.recursive = true
				case 'L':
					opts.dereference = true
				case 'P':
					opts.noDerefer = true
				default:
					// Ignore unrecognized short flags for forward compatibility.
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	if len(operands) < 2 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
		os.Exit(1)
	}

	// R2.4: -P is the default with -r when neither -L nor -P is explicit.
	if opts.recursive && !opts.dereference && !opts.noDerefer {
		opts.noDerefer = true
	}
	// -L overrides -P when both are given (last one wins in GNU, but we
	// resolve: if dereference is set, noDerefer is cleared).
	if opts.dereference {
		opts.noDerefer = false
	}

	// R1.1: last argument is the destination; all preceding are sources.
	dest := operands[len(operands)-1]
	sources := operands[:len(operands)-1]

	destInfo, destErr := os.Stat(dest)
	destIsDir := destErr == nil && destInfo.IsDir()

	// R1.1: multiple sources require destination to be a directory.
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n", programName, dest)
		os.Exit(1)
	}

	exitCode := 0
	for _, src := range sources {
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := copyEntry(src, target, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// copyEntry copies src to dst. For directories, it recurses when opts.recursive
// is set. For symlinks, it either follows or preserves based on opts.
//
// R2.1: recursive directory copying.
// R2.2: without -r, directories produce an error.
// R2.3: -L follows symlinks.
// R2.4: -P preserves symlinks.
func copyEntry(src, dst string, opts cpOpts) error {
	// Use Lstat to detect symlinks before deciding whether to follow them.
	srcLstat, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// Handle symlinks.
	if srcLstat.Mode()&os.ModeSymlink != 0 {
		if opts.noDerefer {
			// R2.4: copy as symlink.
			return copySymlink(src, dst)
		}
		// R2.3: follow the symlink — stat to get the target info.
		srcStat, statErr := os.Stat(src)
		if statErr != nil {
			return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(statErr))
		}
		srcLstat = srcStat
	}

	if srcLstat.IsDir() {
		// R2.2: without -r, refuse to copy a directory.
		if !opts.recursive {
			return fmt.Errorf("-r not specified; omitting directory '%s'", src)
		}
		// R2.1: recursive directory copy.
		return copyDir(src, dst, opts)
	}

	return copyFile(src, dst)
}

// copyDir recursively copies the directory at src to dst.
//
// R2.1: preserves directory structure.
func copyDir(src, dst string, opts cpOpts) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// Create the destination directory with the same permissions.
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("cannot create directory '%s': %s", dst, sysErrMsg(err))
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("cannot open directory '%s' for reading: %s", src, sysErrMsg(err))
	}

	var firstErr error
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if copyErr := copyEntry(srcPath, dstPath, opts); copyErr != nil {
			if firstErr == nil {
				firstErr = copyErr
			} else {
				fmt.Fprintf(os.Stderr, "%s: %v\n", programName, copyErr)
			}
		}
	}
	return firstErr
}

// copySymlink copies a symbolic link from src to dst, preserving the link target.
//
// R2.4: the symlink is recreated at dst pointing to the same target.
func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("cannot read link '%s': %s", src, sysErrMsg(err))
	}
	if err := os.Symlink(target, dst); err != nil {
		return fmt.Errorf("cannot create symbolic link '%s': %s", dst, sysErrMsg(err))
	}
	return nil
}

// copyFile copies the regular file at src to dst using streaming I/O.
// R1.2: preserves file content byte-for-byte.
// R1.3: reports errors for missing source, unwritable destination, etc.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// R1.3: detect source equals destination.
	srcAbs, err1 := filepath.Abs(src)
	dstAbs, err2 := filepath.Abs(dst)
	if err1 == nil && err2 == nil && srcAbs == dstAbs {
		return fmt.Errorf("'%s' and '%s' are the same file", src, dst)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %s", src, sysErrMsg(err))
	}
	defer in.Close() // best-effort cleanup, error ignored

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("cannot create regular file '%s': %s", dst, sysErrMsg(err))
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() // best-effort close before returning error
		return fmt.Errorf("writing '%s': %s", dst, sysErrMsg(err))
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing '%s': %s", dst, sysErrMsg(err))
	}
	return nil
}

// sysErrMsg extracts the underlying syscall error message string from a
// (possibly wrapped) error, producing GNU-compatible messages like
// "No such file or directory" rather than Go's "stat /path: no such file...".
func sysErrMsg(err error) string {
	var msg string
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg = errno.Error()
	} else {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			msg = pathErr.Err.Error()
		} else {
			msg = err.Error()
		}
	}
	return capitalizeFirst(msg)
}

// capitalizeFirst returns s with its first rune uppercased, matching GNU
// coreutils error message capitalization (e.g., "No such file or directory").
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
