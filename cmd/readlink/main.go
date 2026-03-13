// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd050-readlink R1.1–R1.6, R2.1–R2.2
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error output.
const programName = "readlink"

// canonMode controls how readlink resolves paths.
type canonMode int

const (
	// modeNone is the default: print the immediate symlink target.
	modeNone canonMode = iota
	// modeCanon (-f) resolves the full canonical path; the last component need not exist.
	modeCanon
	// modeExisting (-e) resolves the full canonical path; every component must exist.
	modeExisting
	// modeMissing (-m) resolves the full canonical path; no component need exist.
	modeMissing
)

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		operands  []string
		mode      canonMode
		noNewline bool
	)

	// Parse flags and operands.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case arg == "--canonicalize" || arg == "-f":
			// R1.3: -f / --canonicalize.
			mode = modeCanon
		case arg == "--canonicalize-existing" || arg == "-e":
			// R1.4: -e / --canonicalize-existing.
			mode = modeExisting
		case arg == "--canonicalize-missing" || arg == "-m":
			// R1.5: -m / --canonicalize-missing.
			mode = modeMissing
		case arg == "--no-newline" || arg == "-n":
			// R1.6: -n / --no-newline.
			noNewline = true
		case strings.HasPrefix(arg, "--"):
			// R3.2: unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Short flag cluster.
			cluster := arg[1:]
			for _, ch := range cluster {
				switch ch {
				case 'f':
					mode = modeCanon
				case 'e':
					mode = modeExisting
				case 'm':
					mode = modeMissing
				case 'n':
					noNewline = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, ch)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R3.1: no operands is a usage error.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R2.2: when multiple operands are given with -n, -n is ignored and a warning is printed.
	if len(operands) > 1 && noNewline {
		fmt.Fprintf(os.Stderr, "%s: ignoring --no-newline with multiple arguments\n", programName)
		noNewline = false
	}

	// R1.1, R2.1: process each operand in order, track failures.
	exitCode := 0
	for idx, path := range operands {
		result, err := resolve(path, mode)
		if err != nil {
			// R1.2: not a symlink or resolution failed — print error to stderr.
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, unwrapErrMsg(err))
			exitCode = 1
			continue
		}

		// R1.6: suppress trailing newline for a single operand with -n.
		// R2.1: each result on a separate line for multiple operands.
		if noNewline && idx == len(operands)-1 {
			fmt.Print(result)
		} else {
			fmt.Println(result)
		}
	}
	os.Exit(exitCode)
}

// resolve dispatches to the appropriate resolution strategy based on mode.
func resolve(path string, mode canonMode) (string, error) {
	switch mode {
	case modeCanon:
		return resolveCanon(path)
	case modeExisting:
		return resolveCanonExisting(path)
	case modeMissing:
		return resolveCanonMissing(path)
	default:
		return resolveDefault(path)
	}
}

// resolveDefault reads the immediate symlink target (R1.1, R1.2).
func resolveDefault(path string) (string, error) {
	return os.Readlink(path)
}

// resolveCanon resolves the full canonical path following all symlinks.
// The final component need not exist, but its parent directory must (R1.3).
func resolveCanon(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// The last component may not exist. Resolve the parent and append the base.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	if dirErr != nil {
		return "", dirErr
	}

	absDir, dirErr := filepath.Abs(resolvedDir)
	if dirErr != nil {
		return "", dirErr
	}

	return filepath.Join(absDir, base), nil
}

// resolveCanonExisting resolves the full canonical path requiring every component
// to exist (R1.4).
func resolveCanonExisting(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

// resolveCanonMissing resolves the full canonical path without requiring any
// component to exist. Resolves symlinks for the longest existing prefix, then
// appends remaining components (R1.5).
func resolveCanonMissing(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Fast path: full resolution when everything exists.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}

	// Walk up the directory tree to find the deepest existing ancestor.
	dir := absPath
	var tail []string
	for {
		parent := filepath.Dir(dir)
		tail = append(tail, filepath.Base(dir))
		if parent == dir {
			// Reached filesystem root without finding an existing ancestor.
			break
		}
		dir = parent

		resolved, err := filepath.EvalSymlinks(dir)
		if err == nil {
			// Found existing ancestor. Rebuild with remaining components.
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
	}

	// Nothing exists; return the cleaned absolute path.
	return absPath, nil
}

// unwrapErrMsg extracts a user-facing error message from an os error.
// os operations wrap the error in *os.PathError; we unwrap to get the syscall message.
func unwrapErrMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
