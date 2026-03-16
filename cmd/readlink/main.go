// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd050-readlink R1.1-R1.6, R2.1-R2.2, R3.1-R3.2:
// cmd/readlink prints the target of a symbolic link or resolves a path to its
// canonical form via -f, -e, -m canonicalization modes. Supports -n to suppress
// the trailing newline for single operands, -v/--verbose for error reporting,
// and -z/--zero for NUL-delimited output.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in diagnostic output.
const progName = "readlink"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		canonicalize        bool // -f
		canonicalizeExist   bool // -e
		canonicalizeMissing bool // -m
		noNewline           bool // -n
		verbose             bool // -v R1.5: print error messages to stderr
		zero                bool // -z R1.6: NUL-delimited output
		paths               []string
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if arg == "-" {
			paths = append(paths, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--canonicalize":
				// R1.3: resolve full canonical path, dir must exist.
				canonicalize = true
			case "--canonicalize-existing":
				// R1.4: resolve full canonical path, all must exist.
				canonicalizeExist = true
			case "--canonicalize-missing":
				// R1.5: resolve full canonical path, nothing need exist.
				canonicalizeMissing = true
			case "--no-newline":
				// R1.6: suppress trailing newline for single operand.
				noNewline = true
			case "--verbose":
				// R1.5: print error messages to stderr.
				verbose = true
			case "--zero":
				// R1.6: NUL-delimited output.
				zero = true
			default:
				// R3.2: unknown long flag.
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)     //nolint:errcheck // best-effort diagnostic
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "-") {
			// Short flags — process each character.
			for _, c := range arg[1:] {
				switch c {
				case 'f':
					// R1.3: canonicalize.
					canonicalize = true
				case 'e':
					// R1.4: canonicalize-existing.
					canonicalizeExist = true
				case 'm':
					// R1.5: canonicalize-missing.
					canonicalizeMissing = true
				case 'n':
					// R1.6: no trailing newline.
					noNewline = true
				case 'v':
					// R1.5: verbose — print error messages to stderr.
					verbose = true
				case 'z':
					// R1.6: NUL-delimited output.
					zero = true
				default:
					// R3.2: unknown short flag.
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, c)          //nolint:errcheck // best-effort diagnostic
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)  //nolint:errcheck // best-effort diagnostic
					os.Exit(1)
				}
			}
		} else {
			paths = append(paths, arg)
		}
	}

	// R3.1: no operands → usage error to stderr, exit 1.
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)                   //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	canonMode := canonicalize || canonicalizeExist || canonicalizeMissing

	// R2.2: when multiple operands are given, -n is ignored with a warning.
	if noNewline && len(paths) > 1 {
		fmt.Fprintf(os.Stderr, "%s: ignoring --no-newline with multiple arguments\n", progName) //nolint:errcheck // best-effort diagnostic
		noNewline = false
	}

	// R1.6: determine output delimiter — NUL when -z is set, newline otherwise.
	delimiter := "\n"
	if zero {
		delimiter = "\x00"
	}

	exitCode := 0

	for i, p := range paths {
		var result string
		var err error

		if canonMode {
			result, err = resolveCanon(p, canonicalizeExist, canonicalizeMissing)
		} else {
			// R1.1/R1.2: default mode — read symlink target.
			result, err = os.Readlink(p)
		}

		if err != nil {
			// R1.5: in canon mode, always print errors; in default mode, only with -v.
			if canonMode || verbose {
				fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, p, stripPathError(err)) //nolint:errcheck // best-effort diagnostic
			}
			exitCode = 1
			continue
		}

		// Print result with appropriate delimiter.
		if noNewline && i == len(paths)-1 {
			fmt.Fprint(os.Stdout, result) //nolint:errcheck // best-effort output
		} else {
			fmt.Fprint(os.Stdout, result+delimiter) //nolint:errcheck // best-effort output
		}
	}

	os.Exit(exitCode)
}

// resolveCanon canonicalizes a path based on the selected mode.
// R1.3 (-f): all but last component must exist.
// R1.4 (-e): all components must exist.
// R1.5 (-m): no components need exist.
// R3.2: empty string operand is always an error.
func resolveCanon(path string, existing, missing bool) (string, error) {
	if path == "" {
		return "", &os.PathError{Op: "readlink", Path: "", Err: fmt.Errorf("No such file or directory")}
	}
	if missing {
		return resolveMissing(path)
	}
	if existing {
		return resolveExisting(path)
	}
	return resolveDefault(path)
}

// resolveDefault implements -f: resolve symlinks, last component may be missing.
// R3.1: follows symlinks at each component; if the path itself is a symlink
// whose target does not exist, reads the link target and resolves that path.
func resolveDefault(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// If path is a symlink (e.g. dangling), follow it and resolve the target.
	target, linkErr := os.Readlink(path)
	if linkErr == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		return resolveDefault(target)
	}

	// Last component may be missing — resolve parent and append base.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	if dirErr != nil {
		return "", err
	}

	absDir, dirErr := filepath.Abs(resolvedDir)
	if dirErr != nil {
		return "", dirErr
	}

	return filepath.Join(absDir, base), nil
}

// resolveExisting implements -e: all components must exist.
func resolveExisting(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

// resolveMissing implements -m: no components need exist.
// R3.1: follows symlinks; if the path is a dangling symlink, reads the target
// and resolves that path instead.
func resolveMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}

	// If path is a symlink (e.g. dangling), follow it and resolve the target.
	target, linkErr := os.Readlink(abs)
	if linkErr == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(abs), target)
		}
		return resolveMissing(target)
	}

	// Walk up to find the longest existing prefix.
	current := abs
	var suffix []string
	for {
		parent := filepath.Dir(current)
		suffix = append([]string{filepath.Base(current)}, suffix...)
		if parent == current {
			return filepath.Clean(abs), nil
		}
		current = parent
		resolved, err = filepath.EvalSymlinks(current)
		if err == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Clean(filepath.Join(parts...)), nil
		}
	}
}

// stripPathError extracts the inner message from a *os.PathError if possible.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
