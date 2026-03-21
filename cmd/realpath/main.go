// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd049-realpath R1.1–R1.5, R2.1–R2.3, R3.1–R3.3, R4.1–R4.3:
// resolve each path argument to its canonical absolute pathname, with
// -e, -m, -s existence/symlink modes, --relative-to/--relative-base
// relative output, and error handling for missing operands, unknown flags,
// and mixed success/failure across multiple paths.
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
	os.Exit(resolvePaths(paths, opts))
}

// options holds parsed command-line flags.
type options struct {
	mode         resolveMode
	strip        bool   // R1.5: -s/--strip/--no-symlinks
	relativeTo   string // R2.1: --relative-to=DIR
	relativeBase string // R2.2: --relative-base=DIR
	err          error
}

// resolvePaths resolves each path and prints the result.
func resolvePaths(paths []string, opts options) int {
	exitCode := 0
	for _, p := range paths {
		resolved, err := resolvePath(p, opts.mode, opts.strip)
		if err != nil {
			printPathError(p, err)
			exitCode = 1
			continue
		}
		fmt.Println(applyRelative(resolved, opts))
	}
	return exitCode
}

// applyRelative applies --relative-to and --relative-base logic. R2.1–R2.3.
func applyRelative(resolved string, opts options) string {
	if opts.relativeBase == "" && opts.relativeTo == "" {
		return resolved
	}
	if opts.relativeBase != "" && opts.relativeTo != "" {
		return applyBothRelative(resolved, opts)
	}
	if opts.relativeBase != "" {
		return applyRelativeBase(resolved, opts.relativeBase)
	}
	return applyRelativeTo(resolved, opts.relativeTo)
}

// applyRelativeTo computes a path relative to the given directory. R2.1.
func applyRelativeTo(resolved, relativeTo string) string {
	rel, err := filepath.Rel(relativeTo, resolved)
	if err != nil {
		return resolved
	}
	return rel
}

// applyRelativeBase prints relative only if path starts with base. R2.2.
func applyRelativeBase(resolved, relativeBase string) string {
	if !pathStartsWith(resolved, relativeBase) {
		return resolved
	}
	return applyRelativeTo(resolved, relativeBase)
}

// applyBothRelative handles both --relative-to and --relative-base. R2.3.
func applyBothRelative(resolved string, opts options) string {
	if !pathStartsWith(resolved, opts.relativeBase) {
		return resolved
	}
	return applyRelativeTo(resolved, opts.relativeTo)
}

// pathStartsWith checks if resolved path starts with the base directory.
func pathStartsWith(resolved, base string) bool {
	if resolved == base {
		return true
	}
	prefix := base
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(resolved, prefix)
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
	case "-e", "--canonicalize-existing":
		opts.mode = modeExisting
		return true
	case "-m", "--canonicalize-missing":
		opts.mode = modeMissing
		return true
	case "-s", "--strip", "--no-symlinks":
		opts.strip = true
		return true
	}
	if val, ok := parseEqualsFlag(arg, "--relative-to"); ok {
		opts.relativeTo = val
		return true
	}
	if val, ok := parseEqualsFlag(arg, "--relative-base"); ok {
		opts.relativeBase = val
		return true
	}
	return handleUnknown(arg, opts)
}

// parseEqualsFlag extracts the value from --flag=value syntax.
func parseEqualsFlag(arg, prefix string) (string, bool) {
	full := prefix + "="
	if strings.HasPrefix(arg, full) {
		return arg[len(full):], true
	}
	return "", false
}

// handleUnknown rejects unknown flags, passes through positionals. R3.2.
func handleUnknown(arg string, opts *options) bool {
	if strings.HasPrefix(arg, "--") && len(arg) > 2 {
		opts.err = fmt.Errorf("unrecognized option '%s'", arg)
		return true
	}
	if strings.HasPrefix(arg, "-") && len(arg) == 2 {
		opts.err = fmt.Errorf("invalid option -- '%c'", arg[1])
		return true
	}
	return false
}

// resolvePath resolves a single path according to the given mode and strip flag.
func resolvePath(path string, mode resolveMode, strip bool) (string, error) {
	if strip {
		return resolveStrip(path), nil
	}
	switch mode {
	case modeExisting:
		return resolveExisting(path)
	case modeMissing:
		return resolveMissing(path)
	default:
		return resolveDefault(path)
	}
}

// resolveStrip cleans . and .. components and makes path absolute
// without resolving symlinks. R1.5.
func resolveStrip(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absPath)
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
