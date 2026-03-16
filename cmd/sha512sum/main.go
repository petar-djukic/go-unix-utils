// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd033-sha512sum R1.1-R1.4: core SHA-512 digest computation and
// standard GNU output format. Computes SHA-512 digests for files or stdin,
// printing one line per input as 128 lowercase hex characters followed by two
// spaces and the filename. Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU sha512sum format.
const progName = "sha512sum"

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	var exitCode int
	if opts.helpRequested {
		printHelp()
		os.Exit(0)
	}
	if opts.versionRequested {
		printVersion()
		os.Exit(0)
	}

	exitCode = run(files)
	os.Exit(exitCode)
}

// options holds parsed command-line flag state.
type options struct {
	helpRequested    bool
	versionRequested bool
}

// run processes files and returns the exit code.
// R1.1: prints "HASH  FILENAME" for each file.
// R1.2: reads stdin when no files given or "-" specified.
// R1.3: processes multiple files in order, continues on error.
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
			// R1.3: print error to stderr, continue processing remaining files.
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

// hashReader computes the SHA-512 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (128 lowercase hex characters, two spaces, filename).
func hashReader(r io.Reader, name string) error {
	h := sha512.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", h.Sum(nil))
	_, err := fmt.Fprintf(os.Stdout, "%s  %s\n", digest, name)
	return err
}

// parseArgs separates flags from file arguments. Supports --help, --version,
// and -- to end flag parsing.
func parseArgs(args []string) (opts options, files []string) {
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
			opts.helpRequested = true
			return opts, nil
		}
		if arg == "--version" {
			opts.versionRequested = true
			return opts, nil
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags will be expanded in future tasks (R2-R4).
			// For now, treat unknown short flags as file arguments.
			files = append(files, arg)
			continue
		}
		// Not a flag — treat as file argument.
		files = append(files, arg)
	}

	return opts, files
}

// printHelp prints usage information to stdout.
// R1.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: sha512sum [OPTION]... [FILE]...
Print or check SHA512 (512-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary  read in binary mode
  -c, --check   read SHA512 sums from the FILEs and check them
  -t, --text    read in text mode (default)
      --tag     create a BSD-style checksum
      --strict  exit non-zero for improperly formatted checksum lines
  -w, --warn    warn about improperly formatted checksum lines
      --help    display this help and exit
      --version output version information and exit
`)
}

// printVersion prints version information to stdout.
// R1.4: --version prints version information to stdout and exits 0.
func printVersion() {
	fmt.Printf("sha512sum (%s) %s\n", "go-unix-utils", version.Version)
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
