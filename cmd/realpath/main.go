// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/realpath: print resolved absolute paths.
// Implements srd049-realpath R1.1, R1.2, R1.3, R1.4, R1.5.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "realpath"

const helpText = `Usage: realpath [OPTION]... FILE...
Print the resolved absolute file name;
all but the last component must exist

  -e, --canonicalize-existing  all components of the path must exist
  -m, --canonicalize-missing   no path components need exist or be a directory
  -s, --strip, --no-symlinks   don't expand symlinks
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = progName + " (go-unix-utils)"

// resolveMode controls how path resolution handles existence and symlinks.
type resolveMode int

const (
	// modeDefault resolves symlinks; all components must exist (same as -e).
	modeDefault resolveMode = iota
	// modeExisting requires every component to exist; resolves symlinks.
	modeExisting
	// modeMissing does not require any component to exist.
	modeMissing
	// modeStrip resolves . and .. only; does not follow symlinks.
	modeStrip
)

func main() {
	sys.InstallSIGPIPEHandler()

	m, paths, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	exitCode := run(m, paths)
	os.Exit(exitCode)
}

// run resolves each path and prints results. Returns 0 on full success, 1 if any failed.
func run(m resolveMode, paths []string) int {
	exitCode := 0
	for _, p := range paths {
		resolved, err := resolvePath(m, p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
			continue
		}
		fmt.Println(resolved)
	}
	return exitCode
}

// resolvePath resolves a single path according to the given mode.
// R1.1/R1.2: default and -e resolve symlinks and require existence.
// R1.4: -m resolves without requiring existence.
// R1.5: -s cleans . and .. without following symlinks.
func resolvePath(m resolveMode, path string) (string, error) {
	switch m {
	case modeMissing:
		return resolveCanonMissing(path)
	case modeStrip:
		return resolveStrip(path)
	default:
		return resolveCanonExisting(path)
	}
}

// resolveCanonExisting resolves symlinks and verifies all components exist.
// Used for default mode and -e mode (R1.1, R1.2, R1.3).
func resolveCanonExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return resolved, nil
}

// resolveCanonMissing resolves as far as possible without requiring existence (R1.4).
func resolveCanonMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return filepath.Clean(abs), nil
}

// resolveStrip cleans . and .. and makes absolute, without following symlinks (R1.5).
func resolveStrip(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return filepath.Clean(abs), nil
}

// parseArgs processes command-line arguments, returning the mode and path operands.
func parseArgs(args []string) (resolveMode, []string, error) {
	m := modeDefault
	var paths []string
	for _, arg := range args {
		if err := handleFlag(arg, &m); err != nil {
			return 0, nil, err
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			paths = append(paths, arg)
		}
	}
	return m, paths, nil
}

// handleFlag processes a single flag argument, updating the mode.
func handleFlag(arg string, m *resolveMode) error {
	switch arg {
	case "--help":
		fmt.Print(helpText)
		os.Exit(0)
	case "--version":
		fmt.Println(versionText)
		os.Exit(0)
	case "-e", "--canonicalize-existing":
		*m = modeExisting
	case "-m", "--canonicalize-missing":
		*m = modeMissing
	case "-s", "--strip", "--no-symlinks":
		*m = modeStrip
	default:
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
	}
	return nil
}
