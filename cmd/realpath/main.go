// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd049-realpath R1.1-R1.5, R2.1-R2.3, R3.1, R3.3:
// cmd/realpath resolves each command-line path argument to its canonical
// absolute pathname, prints one per line, and reports errors for nonexistent
// paths. Supports -s (no symlink resolution), --relative-to, and
// --relative-base flags. Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in diagnostic output.
const progName = "realpath"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		strip        bool
		relativeTo   string
		relativeBase string
		paths        []string
	)

	// D2: parse flags following the pattern established in R1.1-R1.4.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		switch {
		case arg == "-s" || arg == "--strip" || arg == "--no-symlinks":
			// R1.5: do not resolve symlinks.
			strip = true
		case arg == "--relative-to":
			// R2.1: next argument is the directory.
			i++
			if i < len(args) {
				relativeTo = args[i]
			}
		case strings.HasPrefix(arg, "--relative-to="):
			// R2.1: value after '='.
			relativeTo = arg[len("--relative-to="):]
		case arg == "--relative-base":
			// R2.2: next argument is the directory.
			i++
			if i < len(args) {
				relativeBase = args[i]
			}
		case strings.HasPrefix(arg, "--relative-base="):
			// R2.2: value after '='.
			relativeBase = arg[len("--relative-base="):]
		default:
			paths = append(paths, arg)
		}
	}

	// R3.1: no operands → usage error to stderr, exit 1.
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)                   //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	// Resolve the --relative-to and --relative-base directories themselves.
	if relativeTo != "" {
		if r, err := resolvePath(relativeTo, strip); err == nil {
			relativeTo = r
		}
	}
	if relativeBase != "" {
		if r, err := resolvePath(relativeBase, strip); err == nil {
			relativeBase = r
		}
	}

	// R2.2: when only --relative-base is given, it also serves as --relative-to.
	if relativeTo == "" && relativeBase != "" {
		relativeTo = relativeBase
	}

	exitCode := 0

	for _, arg := range paths {
		resolved, err := resolvePath(arg, strip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, arg, err) //nolint:errcheck // best-effort diagnostic
			exitCode = 1
			continue
		}

		output := makeRelative(resolved, relativeTo, relativeBase)
		fmt.Fprintln(os.Stdout, output) //nolint:errcheck // best-effort output
	}

	os.Exit(exitCode)
}

// resolvePath resolves a path using the appropriate mode.
// When strip is true (R1.5), symlinks are not resolved.
func resolvePath(path string, strip bool) (string, error) {
	if strip {
		return resolveStrip(path)
	}
	return resolve(path)
}

// resolve canonicalizes path to its absolute form with symlinks resolved.
// R1.1: GNU realpath default behavior requires all parent components to exist
// but allows the final component to be missing. It resolves symlinks in the
// existing prefix and appends the remaining base name.
func resolve(path string) (string, error) {
	// Try full resolution first — works when the entire path exists.
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// R1.1/R1.2: if the full path doesn't exist, resolve the parent directory
	// (which must exist) and append the base name. This matches GNU realpath
	// default behavior where the last component may be missing.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	if dirErr != nil {
		// Parent doesn't exist — return the original error.
		return "", err
	}

	absDir, dirErr := filepath.Abs(resolvedDir)
	if dirErr != nil {
		return "", dirErr
	}

	return filepath.Join(absDir, base), nil
}

// resolveStrip cleans . and .. components and makes the path absolute without
// resolving symlinks (R1.5).
func resolveStrip(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// makeRelative applies --relative-to and --relative-base logic to a resolved path.
// R2.1: when relativeTo is set and relativeBase is empty, all paths are relative.
// R2.2: when relativeBase is set, only paths under relativeBase are relative.
// R2.3: when both are set, relativeBase constrains and relativeTo is the base.
func makeRelative(resolved, relativeTo, relativeBase string) string {
	if relativeTo == "" {
		return resolved
	}

	if relativeBase != "" {
		// R2.2/R2.3: only relativize if the resolved path is below relativeBase.
		if !hasPathPrefix(resolved, relativeBase) {
			return resolved
		}
	}

	rel, err := filepath.Rel(relativeTo, resolved)
	if err != nil {
		return resolved
	}
	return rel
}

// hasPathPrefix reports whether path is equal to or a subdirectory of prefix.
func hasPathPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}
