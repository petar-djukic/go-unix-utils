// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd056-cp R1.1, R1.2, R1.3, R1.4
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.5 equivalent: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "cp: missing file operand\n")
		os.Exit(1)
	}

	// R1.1: last argument is the destination; all preceding are sources.
	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	destInfo, destErr := os.Stat(dest)
	destIsDir := destErr == nil && destInfo.IsDir()

	// R1.1: multiple sources require destination to be a directory.
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr, "cp: target '%s' is not a directory\n", dest)
		os.Exit(1)
	}

	exitCode := 0
	for _, src := range sources {
		target := dest
		// R1.4: when destination is an existing directory, use source basename.
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := copyFile(src, target); err != nil {
			fmt.Fprintf(os.Stderr, "cp: %v\n", err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// copyFile copies the regular file at src to dst using streaming I/O.
// R1.2: preserves file content byte-for-byte.
// R1.3: reports errors for missing source, unwritable destination, etc.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("-r not specified; omitting directory '%s'", src)
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

	// D3: use os.Create + io.Copy for streaming copy without reading entire file.
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
