// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd050-readlink R1.1–R1.4: print symlink target or canonical
// path with -f (--canonicalize) and -e (--canonicalize-existing) modes.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "readlink"

type resolveMode int

const (
	modeReadlink resolveMode = iota // default: print symlink target
	modeCanon                       // -f: canonicalize, last may be missing
	modeExisting                    // -e: canonicalize, all must exist
)

func main() {
	sys.InstallSIGPIPEHandler()
	opts, paths := parseArgs(os.Args[1:])
	if opts.err != nil {
		printUsageError(opts.err.Error())
		os.Exit(1)
	}
	if len(paths) == 0 {
		printUsageError("missing operand")
		os.Exit(1)
	}
	os.Exit(processArgs(paths, opts))
}

// options holds parsed command-line flags.
type options struct {
	mode resolveMode
	err  error
}

// processArgs processes each path argument and prints results.
// GNU readlink silently exits 1 on path errors — no stderr output.
func processArgs(paths []string, opts options) int {
	exitCode := 0
	for _, p := range paths {
		result, err := resolvePath(p, opts.mode)
		if err != nil {
			exitCode = 1
			continue
		}
		fmt.Println(result)
	}
	return exitCode
}

// resolvePath dispatches to the appropriate resolver based on mode.
func resolvePath(path string, mode resolveMode) (string, error) {
	switch mode {
	case modeCanon:
		return resolveCanon(path)
	case modeExisting:
		return resolveExisting(path)
	default:
		return resolveReadlink(path)
	}
}

// resolveReadlink reads the immediate symlink target. R1.1, R1.2.
func resolveReadlink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	return target, nil
}

// resolveCanon resolves the full canonical path. The last component need
// not exist but its parent directory must. R1.3.
func resolveCanon(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}
	return resolveAllButLast(absPath)
}

// resolveAllButLast resolves the parent directory and appends the last
// component. The parent must exist; the last component may not.
func resolveAllButLast(absPath string) (string, error) {
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// resolveExisting resolves the full canonical path. All components
// must exist. R1.4.
func resolveExisting(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}

// parseArgs splits raw arguments into options and positional paths.
func parseArgs(args []string) (options, []string) {
	var opts options
	var paths []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if parseFlag(arg, &opts) {
			if opts.err != nil {
				return opts, nil
			}
			continue
		}
		paths = append(paths, arg)
	}
	return opts, paths
}

// parseFlag parses a single flag argument. Returns true if consumed.
func parseFlag(arg string, opts *options) bool {
	switch arg {
	case "-f", "--canonicalize":
		opts.mode = modeCanon
		return true
	case "-e", "--canonicalize-existing":
		opts.mode = modeExisting
		return true
	}
	return handleUnknown(arg, opts)
}

// handleUnknown rejects unknown flags, passes through positionals.
func handleUnknown(arg string, opts *options) bool {
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		opts.err = fmt.Errorf("unrecognized option '%s'", arg)
		return true
	}
	return false
}

// printUsageError writes a usage error to stderr.
func printUsageError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}
