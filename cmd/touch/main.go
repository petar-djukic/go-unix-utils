// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/touch: create files and update timestamps.
// Implements srd062-touch R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "touch"

const tryHelp = "Try 'touch --help' for more information."

// helpText is the usage message printed for --help.
const helpText = `Usage: touch [OPTION]... FILE...
Update the access and modification times of each FILE to the current time.

A FILE argument that does not exist is created empty, unless -c or -h
is supplied.

      -a                     change only the access time
      -c, --no-create        do not create any files
      -d, --date=STRING      parse STRING and use it instead of current time
      -h, --no-dereference   affect each symbolic link instead of any referenced
                             file (useful only on systems that can change the
                             timestamps of a symlink)
      -m                     change only the modification time
      -r, --reference=FILE   use this file's times instead of current time
      -t STAMP               use [[CC]YY]MMDDhhmm[.ss] instead of current time
      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version string printed for --version.
const versionText = "touch (go-unix-utils) 1.0\n"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// options holds parsed command-line flags.
type options struct {
	noCreate bool
	files    []string
}

// run executes the touch logic and returns the exit code.
// R1.1: update access and modification times to current time.
// R1.2: create file if absent. R1.3: -c suppresses creation.
// R1.4: process multiple file arguments in order.
func run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", progName, err, tryHelp)
		return 1
	}
	if opts == nil {
		return 0 // --help or --version handled
	}
	if len(opts.files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n%s\n", progName, tryHelp)
		return 1
	}
	exitCode := 0
	now := time.Now()
	for _, f := range opts.files {
		if err := touchFile(f, now, opts.noCreate); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs parses command-line arguments into options.
// Returns nil options when --help or --version was handled.
func parseArgs(args []string) (*options, error) {
	opts := &options{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			opts.files = append(opts.files, args[i:]...)
			return opts, nil
		}
		if arg == "--help" {
			fmt.Fprint(os.Stdout, helpText)
			return nil, nil
		}
		if arg == "--version" {
			fmt.Fprint(os.Stdout, versionText)
			return nil, nil
		}
		if arg == "--no-create" {
			opts.noCreate = true
			i++
			continue
		}
		if handled, advance, err := parseLongWithValue(arg, args, i); handled {
			if err != nil {
				return nil, err
			}
			i += advance
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			advance, err := parseShortFlags(arg[1:], args, i, opts)
			if err != nil {
				return nil, err
			}
			i += advance
			continue
		}
		opts.files = append(opts.files, arg)
		i++
	}
	return opts, nil
}

// parseLongWithValue handles --date=X and --reference=X flags.
// Returns (handled, advance, error). These are R2/R3 flags; we
// accept them for parsing but they are no-ops in this R1 scope.
func parseLongWithValue(arg string, args []string, idx int) (bool, int, error) {
	if strings.HasPrefix(arg, "--date=") || strings.HasPrefix(arg, "--reference=") {
		return true, 1, nil
	}
	if arg == "--date" || arg == "--reference" {
		if idx+1 >= len(args) {
			return true, 1, fmt.Errorf("option '%s' requires an argument", arg)
		}
		return true, 2, nil
	}
	return false, 0, nil
}

// parseShortFlags processes a cluster of short flags (e.g., "-acm").
// R1.3: -c sets noCreate.
func parseShortFlags(flags string, args []string, idx int, opts *options) (int, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a', 'm':
			// R2.1, R2.2: -a and -m flags; accepted for parsing, no-op in R1 scope.
		case 'c':
			opts.noCreate = true
		case 'h':
			// R3.4: --no-dereference; accepted for parsing, no-op in R1 scope.
		case 'r':
			// -r FILE: skip the value argument.
			if j+1 < len(flags) {
				return 1, nil // rest of cluster is the value
			}
			if idx+1 >= len(args) {
				return 1, fmt.Errorf("option requires an argument -- 'r'")
			}
			return 2, nil
		case 't':
			// -t STAMP: skip the value argument.
			if j+1 < len(flags) {
				return 1, nil
			}
			if idx+1 >= len(args) {
				return 1, fmt.Errorf("option requires an argument -- 't'")
			}
			return 2, nil
		case 'd':
			// -d STRING: skip the value argument.
			if j+1 < len(flags) {
				return 1, nil
			}
			if idx+1 >= len(args) {
				return 1, fmt.Errorf("option requires an argument -- 'd'")
			}
			return 2, nil
		default:
			return 1, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// touchFile updates timestamps or creates a file.
// R1.1: update access and modification times to now.
// R1.2: create file if it does not exist.
// R1.3: skip creation when noCreate is true.
func touchFile(path string, now time.Time, noCreate bool) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if noCreate {
			return nil // R1.3: suppress creation silently
		}
		return createFile(path, now)
	}
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	return updateTimestamps(path, now)
}

// createFile creates an empty file and sets its timestamps.
// R1.2: create file as empty with default permissions.
func createFile(path string, now time.Time) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	return os.Chtimes(path, now, now)
}

// updateTimestamps sets both access and modification times.
// R1.1: update both times to the current time.
func updateTimestamps(path string, now time.Time) error {
	if err := os.Chtimes(path, now, now); err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	return nil
}
