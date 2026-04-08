// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/readlink: print symlink target or canonical path.
// Implements srd050-readlink R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R2.1, R2.2.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "readlink"

const helpText = `Usage: readlink [OPTION]... FILE...
Print value of a symbolic link or canonical file name

  -f, --canonicalize            canonicalize by following every symlink in
                                every component of the given name recursively;
                                all but the last component must exist
  -e, --canonicalize-existing   canonicalize by following every symlink in
                                every component of the given name recursively,
                                all components must exist
  -m, --canonicalize-missing    canonicalize by following every symlink in
                                every component of the given name recursively,
                                without requirements on components existence
  -n, --no-newline              do not output the trailing delimiter
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = progName + " (go-unix-utils)"

// canonMode controls how path resolution handles existence and symlinks.
type canonMode int

const (
	// modeDefault prints the immediate symlink target without canonicalization.
	modeDefault canonMode = iota
	// modeCanon resolves all symlinks; all but the last component must exist (-f).
	modeCanon
	// modeExisting resolves all symlinks; every component must exist (-e).
	modeExisting
	// modeMissing resolves all symlinks; no component need exist (-m).
	modeMissing
)

// options holds parsed command-line options.
type options struct {
	mode      canonMode
	noNewline bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, operands, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	os.Exit(run(opts, operands))
}

// run processes each operand and returns the exit code.
func run(opts options, operands []string) int {
	exitCode := 0
	for _, op := range operands {
		result, err := processOperand(opts.mode, op)
		if err != nil {
			// R1.2: default mode prints nothing on failure.
			if opts.mode != modeDefault {
				printError(op, err)
			}
			exitCode = 1
			continue
		}
		// R2.2: -n is ignored when multiple operands are given.
		suppressNL := opts.noNewline && len(operands) == 1
		printResult(result, suppressNL)
	}
	return exitCode
}

// printError writes a formatted error message to stderr.
func printError(path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, path, err)
}

// printResult writes a resolved path to stdout.
func printResult(result string, suppressNewline bool) {
	if suppressNewline {
		fmt.Print(result)
	} else {
		fmt.Println(result)
	}
}

// processOperand resolves a single operand according to the canonicalization mode.
func processOperand(m canonMode, path string) (string, error) {
	switch m {
	case modeCanon:
		return resolveCanon(path)
	case modeExisting:
		return resolveExisting(path)
	case modeMissing:
		return resolveMissing(path)
	default:
		return readSymlink(path)
	}
}

// readSymlink returns the immediate target of a symbolic link (R1.1).
// Returns an error if the operand is not a symlink (R1.2).
func readSymlink(path string) (string, error) {
	return os.Readlink(path)
}

// resolveCanon resolves with -f semantics (R1.3): follow all symlinks,
// all but the last component must exist.
func resolveCanon(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Try resolving the full path including the last component.
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	// Last component may not exist; resolve the parent directory.
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// resolveExisting resolves with -e semantics (R1.4): follow all symlinks,
// every component must exist.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveMissing resolves with -m semantics (R1.5): follow all symlinks,
// no component need exist.
func resolveMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return resolvePartial(abs), nil
}

// resolvePartial resolves symlinks for existing path components and
// preserves non-existent trailing components.
func resolvePartial(abs string) string {
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)
	if dir == abs {
		return abs
	}
	return filepath.Join(resolvePartial(dir), base)
}

// parseArgs processes command-line arguments into options and operands.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var operands []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if err := handleFlag(arg, &opts); err != nil {
			return options{}, nil, err
		}
	}
	return opts, operands, nil
}

// handleFlag processes a single flag argument, updating options.
func handleFlag(arg string, opts *options) error {
	switch arg {
	case "-f", "--canonicalize":
		opts.mode = modeCanon
	case "-e", "--canonicalize-existing":
		opts.mode = modeExisting
	case "-m", "--canonicalize-missing":
		opts.mode = modeMissing
	case "-n", "--no-newline":
		opts.noNewline = true
	case "--help":
		fmt.Print(helpText)
		os.Exit(0)
	case "--version":
		fmt.Println(versionText)
		os.Exit(0)
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}
