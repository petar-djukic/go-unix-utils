// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd049-realpath R1.1–R1.5, R2.1–R2.3
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "realpath"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		operands     []string
		stripMode    bool
		relativeTo   string
		relativeBase string
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case arg == "--strip" || arg == "--no-symlinks":
			// R1.5: do not resolve symlinks.
			stripMode = true
		case arg == "--relative-to":
			// R2.1: --relative-to DIR (space-separated form).
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n", programName, arg)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
				os.Exit(1)
			}
			i++
			relativeTo = args[i]
		case strings.HasPrefix(arg, "--relative-to="):
			// R2.1: --relative-to=DIR (equals form).
			relativeTo = arg[len("--relative-to="):]
		case arg == "--relative-base":
			// R2.2: --relative-base DIR (space-separated form).
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n", programName, arg)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
				os.Exit(1)
			}
			i++
			relativeBase = args[i]
		case strings.HasPrefix(arg, "--relative-base="):
			// R2.2: --relative-base=DIR (equals form).
			relativeBase = arg[len("--relative-base="):]
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
				case 's':
					// R1.5: -s (short form of --strip).
					stripMode = true
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

	// Resolve --relative-to and --relative-base using the same mode as operands.
	userSetRelTo := relativeTo != ""
	userSetRelBase := relativeBase != ""

	var resolvedRelTo, resolvedRelBase string
	if userSetRelTo {
		var err error
		resolvedRelTo, err = resolveInMode(relativeTo, stripMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: No such file or directory\n", programName, relativeTo)
			os.Exit(1)
		}
	}
	if userSetRelBase {
		var err error
		resolvedRelBase, err = resolveInMode(relativeBase, stripMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: No such file or directory\n", programName, relativeBase)
			os.Exit(1)
		}
	}

	// R2.2: when only --relative-base is set, --relative-to defaults to --relative-base.
	if userSetRelBase && !userSetRelTo {
		resolvedRelTo = resolvedRelBase
	}

	// R1.1, R1.2, R3.3: resolve each path and print one per line.
	exitCode := 0
	for _, path := range operands {
		resolved, err := resolveInMode(path, stripMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: No such file or directory\n", programName, path)
			exitCode = 1
			continue
		}

		output := resolved
		if userSetRelTo || userSetRelBase {
			if userSetRelBase {
				// R2.2/R2.3: only relativize if path starts with resolved base.
				if pathStartsWith(resolved, resolvedRelBase) {
					output = makeRelative(resolved, resolvedRelTo)
				}
			} else {
				// R2.1: always relativize when only --relative-to is set.
				output = makeRelative(resolved, resolvedRelTo)
			}
		}

		fmt.Println(output)
	}
	os.Exit(exitCode)
}

// resolveInMode resolves a path according to the current mode flags.
func resolveInMode(path string, strip bool) (string, error) {
	if strip {
		return resolveStrip(path)
	}
	return resolvePath(path)
}

// resolvePath resolves a path to its canonical absolute form with all symlinks resolved.
// GNU realpath default mode requires all but the last component to exist. If the full
// path exists, resolve it directly. Otherwise, resolve the parent directory and append
// the final component.
//
// R1.1: resolve symlinks and produce absolute canonical path.
func resolvePath(path string) (string, error) {
	// Try resolving the full path first.
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// Default mode: all but the last component must exist.
	// Resolve the parent directory and append the base name.
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

// resolveStrip resolves a path without following symlinks.
// R1.5: only clean . and .. components and prepend the working directory for relative paths.
// In default mode (no -e/-m), all but the last component must exist.
// Existence is checked via os.Lstat (symlinks are not followed).
func resolveStrip(path string) (string, error) {
	// Make absolute without cleaning (filepath.Abs calls Clean, which removes ..).
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = wd + "/" + path
	}

	// Split into path components, preserving . and .. for the walk.
	components := strings.Split(path, "/")
	var parts []string
	for _, c := range components {
		if c != "" {
			parts = append(parts, c)
		}
	}

	if len(parts) == 0 {
		return "/", nil
	}

	var resolved []string
	lastIdx := len(parts) - 1

	for i, part := range parts {
		isLast := i == lastIdx

		switch part {
		case ".":
			// No-op: stay in current directory.
		case "..":
			// Go up one level.
			if len(resolved) > 0 {
				resolved = resolved[:len(resolved)-1]
			}
		default:
			resolved = append(resolved, part)
			// Default mode: all but the last component must exist.
			if !isLast {
				checkPath := "/" + strings.Join(resolved, "/")
				if _, err := os.Lstat(checkPath); err != nil {
					return "", err
				}
			}
		}
	}

	if len(resolved) == 0 {
		return "/", nil
	}
	return "/" + strings.Join(resolved, "/"), nil
}

// pathStartsWith returns true if path equals prefix or is a subdirectory of prefix.
func pathStartsWith(path, prefix string) bool {
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// makeRelative computes the relative path from base to target.
func makeRelative(target, base string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// printHelp writes usage information to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: realpath [OPTION]... FILE...
Print the resolved absolute file name;
all but the last component must exist

  -s, --strip, --no-symlinks  don't expand symlinks
      --relative-to=DIR       print the resolved path relative to DIR
      --relative-base=DIR     print absolute paths unless paths below DIR
      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
func printVersion() {
	fmt.Println("realpath (go-unix-utils) 0.1")
}
