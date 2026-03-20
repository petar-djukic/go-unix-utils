// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd049-realpath R1.1–R1.4: resolve each path argument
// to its canonical absolute pathname, with -e and -m existence modes.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "realpath"

type resolveMode int

const (
	modeDefault  resolveMode = iota // all components except last must exist
	modeExisting                    // -e: all components must exist
	modeMissing                     // -m: no component needs to exist
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
	os.Exit(resolvePaths(paths, opts.mode))
}

// options holds parsed command-line flags.
type options struct {
	mode resolveMode
	err  error
}

// resolvePaths resolves each path and prints the result.
func resolvePaths(paths []string, mode resolveMode) int {
	exitCode := 0
	for _, p := range paths {
		resolved, err := resolvePath(p, mode)
		if err != nil {
			printPathError(p, err)
			exitCode = 1
			continue
		}
		fmt.Println(resolved)
	}
	return exitCode
}

// printUsageError writes a usage error to stderr. R3.1.
func printUsageError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// printPathError writes a path resolution error to stderr. R1.2, R3.3.
func printPathError(path string, err error) {
	msg := err.Error()
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		msg = pathErr.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, msg)
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
		switch arg {
		case "-e", "--canonicalize-existing":
			opts.mode = modeExisting
		case "-m", "--canonicalize-missing":
			opts.mode = modeMissing
		default:
			if strings.HasPrefix(arg, "-") && len(arg) > 1 {
				opts.err = fmt.Errorf("unrecognized option '%s'", arg)
				return opts, nil
			}
			paths = append(paths, arg)
		}
	}
	return opts, paths
}

// resolvePath resolves a single path according to the given mode.
func resolvePath(path string, mode resolveMode) (string, error) {
	switch mode {
	case modeExisting:
		return resolveExisting(path)
	case modeMissing:
		return resolveMissing(path)
	default:
		return resolveDefault(path)
	}
}

// resolveDefault resolves a path where all components except the last
// must exist (CAN_ALL_BUT_LAST). R1.1, R1.2.
func resolveDefault(path string) (string, error) {
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

// resolveAllButLast resolves the parent directory and appends the
// last component. The parent must exist; the last component may not.
func resolveAllButLast(absPath string) (string, error) {
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// resolveExisting resolves a path where all components must exist. R1.3.
func resolveExisting(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}

// resolveMissing resolves a path where no component needs to exist. R1.4.
func resolveMissing(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}
	return resolvePartial(absPath), nil
}

// resolvePartial walks an absolute path component by component,
// resolving symlinks for existing components and appending missing ones.
func resolvePartial(absPath string) string {
	trimmed := strings.TrimPrefix(absPath, string(filepath.Separator))
	if trimmed == "" {
		return string(filepath.Separator)
	}
	components := strings.Split(trimmed, string(filepath.Separator))
	current := string(filepath.Separator)
	for _, comp := range components {
		next := filepath.Join(current, comp)
		resolved, err := filepath.EvalSymlinks(next)
		if err != nil {
			current = next
		} else {
			current = resolved
		}
	}
	return current
}
