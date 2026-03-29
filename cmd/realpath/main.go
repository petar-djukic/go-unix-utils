// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/realpath implements GNU realpath: print resolved absolute paths.
//
// Implements prd049-realpath R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "realpath"

// mode represents the existence-checking mode for path resolution.
type mode int

const (
	// R1.1, R1.2: resolve all but last component; last may be missing.
	modeDefault mode = iota
	// R1.3: -e, every component must exist.
	modeStrict
	// R1.4: -m, no component needs to exist.
	modeMissing
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and resolves paths. Returns 0 on success, 1 on error.
func run(args []string, stdout, stderr *os.File) int {
	m, paths, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		return 1
	}
	if len(paths) == 0 {
		printMissingOperand(stderr)
		return 1
	}

	exitCode := 0
	for _, p := range paths {
		resolved, resolveErr := resolvePath(p, m)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, resolveErr) //nolint:errcheck
			exitCode = 1
			continue
		}
		fmt.Fprintln(stdout, resolved) //nolint:errcheck
	}
	return exitCode
}

// printMissingOperand writes the missing-operand error to stderr.
func printMissingOperand(stderr *os.File) {
	fmt.Fprintf(stderr, "%s: missing operand\n", progName)                   //nolint:errcheck
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// parseArgs extracts the mode and path operands from command-line arguments.
func parseArgs(args []string) (mode, []string, error) {
	m := modeDefault
	var paths []string
	endOfFlags := false

	for i := range len(args) {
		arg := args[i]
		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if parsed, err := parseLong(arg, &m); parsed {
			if err != nil {
				return m, nil, err
			}
			continue
		}
		if err := parseShort(arg, &m); err != nil {
			return m, nil, err
		}
	}
	return m, paths, nil
}

// parseLong handles long flags. Returns true if the arg was a long flag.
func parseLong(arg string, m *mode) (bool, error) {
	switch arg {
	case "--canonicalize-existing":
		*m = modeStrict
		return true, nil
	case "--canonicalize-missing":
		*m = modeMissing
		return true, nil
	}
	if strings.HasPrefix(arg, "--") {
		return true, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return false, nil
}

// parseShort handles short flag bundles like -em.
func parseShort(arg string, m *mode) error {
	for _, ch := range arg[1:] {
		switch ch {
		case 'e':
			*m = modeStrict
		case 'm':
			*m = modeMissing
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// resolvePath resolves a single path according to the given mode.
func resolvePath(p string, m mode) (string, error) {
	switch m {
	case modeMissing:
		return resolveMissing(p)
	case modeStrict:
		return resolveStrict(p)
	default:
		return resolveDefault(p)
	}
}

// resolveDefault resolves symlinks for all but the last component (R1.1, R1.2).
// The parent directory must exist; the last component may be missing.
func resolveDefault(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%s: %s", p, errMessage(err))
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)

	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %s", p, errMessage(err))
	}
	// If the full path exists, resolve its symlinks too.
	full := filepath.Join(resolvedDir, base)
	if resolved, evalErr := filepath.EvalSymlinks(full); evalErr == nil {
		return resolved, nil
	}
	return full, nil
}

// resolveStrict resolves symlinks and requires every component to exist (R1.3).
func resolveStrict(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("%s: %s", p, errMessage(err))
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("%s: %s", p, errMessage(err))
	}
	return abs, nil
}

// resolveMissing resolves as far as possible, constructs the rest (R1.4).
func resolveMissing(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%s: %s", p, errMessage(err))
	}
	return resolveExistingPrefix(abs), nil
}

// resolveExistingPrefix walks from the root, resolving symlinks for
// components that exist and appending the rest literally.
func resolveExistingPrefix(abs string) string {
	parts := strings.Split(abs, string(filepath.Separator))
	resolved := "/"
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		candidate := filepath.Join(resolved, parts[i])
		real, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			remaining := filepath.Join(parts[i:]...)
			return filepath.Join(resolved, remaining)
		}
		resolved = real
	}
	return resolved
}

// errMessage extracts the inner message from a *os.PathError or returns
// the full error string.
func errMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
