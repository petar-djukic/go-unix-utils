// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd032-sha256sum R1.1-R1.4: core SHA-256 digest computation,
// standard GNU output format, stdin reading, multiple file processing with
// error handling, and --version/--help flags. Computes SHA-256 digests for
// files or stdin, printing one line per input in text mode (default).
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU sha256sum format.
const progName = "sha256sum"

func main() {
	sys.InstallSIGPIPEHandler()

	files := parseArgs(os.Args[1:])

	exitCode := run(files)
	os.Exit(exitCode)
}

// run processes files and returns the exit code.
func run(files []string) int {
	exitCode := 0

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-"); err != nil {
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
			if err := hashReader(os.Stdin, "-"); err != nil {
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
		if err := hashReader(f, name); err != nil {
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

// hashReader computes the SHA-256 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (text mode, default).
func hashReader(r io.Reader, name string) error {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", h.Sum(nil))

	_, err := fmt.Fprintf(os.Stdout, "%s  %s\n", digest, name)
	return err
}

// parseArgs separates flags from file arguments. Supports --help, --version,
// and -- to end flag parsing.
func parseArgs(args []string) (files []string) {
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
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags not yet supported in R1 scope; treat as file arg.
			files = append(files, arg)
			continue
		}
		// Not a flag — treat as file argument.
		files = append(files, arg)
	}

	return files
}

// printHelp prints usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: sha256sum [OPTION]... [FILE]...
Print or check SHA256 (256-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary  read in binary mode
  -c, --check   read SHA256 sums from the FILEs and check them
  -t, --text    read in text mode (default)
      --tag     create a BSD-style checksum
      --strict  exit non-zero for improperly formatted checksum lines
  -w, --warn    warn about improperly formatted checksum lines
      --help    display this help and exit
      --version output version information and exit
`)
}

// printVersion prints version information to stdout.
func printVersion() {
	fmt.Printf("sha256sum (%s) %s\n", "go-unix-utils", version.Version)
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
