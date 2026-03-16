// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd030-md5sum R1.1-R1.4: core MD5 digest computation and standard
// GNU output format. Computes MD5 digests for files or stdin, printing one line
// per input in text or binary mode format. Installs SIGPIPE handler for clean
// exit on broken pipe (R4.3).
package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU md5sum format.
const progName = "md5sum"

func main() {
	sys.InstallSIGPIPEHandler()

	binaryMode, files := parseArgs(os.Args[1:])
	exitCode := run(binaryMode, files)
	os.Exit(exitCode)
}

// run processes files and returns the exit code.
func run(binaryMode bool, files []string) int {
	exitCode := 0

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-", binaryMode); err != nil {
			if isEPIPE(err) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
			return 1
		}
		return 0
	}

	// R1.3: process multiple file arguments sequentially.
	for _, name := range files {
		if name == "-" {
			// R1.2: "-" means read from stdin.
			if err := hashReader(os.Stdin, "-", binaryMode); err != nil {
				if isEPIPE(err) {
					os.Exit(0)
				}
				fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			// R1.4: print error to stderr, continue processing remaining files.
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		if err := hashReader(f, name, binaryMode); err != nil {
			f.Close() // best-effort close
			if isEPIPE(err) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
			continue
		}
		f.Close() // best-effort close
	}

	return exitCode
}

// hashReader computes the MD5 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (text mode) or "HASH *FILENAME" (binary mode).
func hashReader(r io.Reader, name string, binaryMode bool) error {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", h.Sum(nil))

	// R1.4/R3.1-R3.2: text mode uses two spaces; binary mode uses space+asterisk.
	sep := "  "
	if binaryMode {
		sep = " *"
	}

	_, err := fmt.Fprintf(os.Stdout, "%s%s%s\n", digest, sep, name)
	return err
}

// parseArgs separates flags from file arguments. Supports -b/--binary,
// -t/--text, and -- to end flag parsing. Single-char flags can be grouped.
func parseArgs(args []string) (binaryMode bool, files []string) {
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--binary" {
			binaryMode = true
			continue
		}
		if arg == "--text" {
			binaryMode = false
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: -b, -t, or grouped like -bt.
			for _, ch := range arg[1:] {
				switch ch {
				case 'b':
					binaryMode = true
				case 't':
					binaryMode = false
				}
			}
			continue
		}
		// Not a flag — treat as file argument.
		files = append(files, arg)
	}

	return binaryMode, files
}

// isEPIPE returns true if err wraps a syscall.EPIPE error.
func isEPIPE(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EPIPE
	}
	return false
}

// unwrapPathError extracts the inner error from an *os.PathError.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
