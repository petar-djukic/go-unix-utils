// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/realpath: print resolved absolute paths.
// Implements srd049-realpath R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
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
      --relative-to=DIR        print the resolved path relative to DIR
      --relative-base=DIR      print absolute paths unless paths below DIR
  -z, --zero                   end each output line with NUL, not newline
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

// options holds all parsed command-line options.
type options struct {
	mode         resolveMode
	relativeTo   string
	relativeBase string
	zero         bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, paths, err := parseArgs(os.Args[1:])
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

	exitCode := run(opts, paths)
	os.Exit(exitCode)
}

// run resolves each path and prints results. Returns 0 on full success, 1 if any failed.
func run(opts options, paths []string) int {
	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}

	exitCode := 0
	for _, p := range paths {
		resolved, err := resolvePath(opts.mode, p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
			continue
		}
		output := applyRelative(resolved, opts)
		fmt.Print(output + terminator)
	}
	return exitCode
}

// applyRelative applies --relative-to and --relative-base to a resolved path.
// R2.1: --relative-to prints relative to DIR.
// R2.2: --relative-base prints relative only if path starts with base.
// R2.3: When both set, --relative-to applies only if path starts with base.
func applyRelative(resolved string, opts options) string {
	if opts.relativeBase != "" && opts.relativeTo != "" {
		return applyBothRelative(resolved, opts)
	}
	if opts.relativeBase != "" {
		return applyRelativeBase(resolved, opts.relativeBase)
	}
	if opts.relativeTo != "" {
		return computeRelative(resolved, opts.relativeTo)
	}
	return resolved
}

// applyBothRelative handles the case where both --relative-to and --relative-base are set.
func applyBothRelative(resolved string, opts options) string {
	if !isUnderDir(resolved, opts.relativeBase) {
		return resolved
	}
	return computeRelative(resolved, opts.relativeTo)
}

// applyRelativeBase handles --relative-base without --relative-to.
func applyRelativeBase(resolved, base string) string {
	if !isUnderDir(resolved, base) {
		return resolved
	}
	return computeRelative(resolved, base)
}

// isUnderDir returns true if path equals dir or is a descendant of dir.
func isUnderDir(path, dir string) bool {
	cleanDir := filepath.Clean(dir)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanDir {
		return true
	}
	return strings.HasPrefix(cleanPath, cleanDir+string(filepath.Separator))
}

// computeRelative returns path relative to base using filepath.Rel.
func computeRelative(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
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

// parseArgs processes command-line arguments, returning options and path operands.
func parseArgs(args []string) (options, []string, error) {
	opts := options{}
	var paths []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			paths = append(paths, arg)
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
	return opts, paths, nil
}

// handleFlag processes a single flag argument, updating options.
func handleFlag(arg string, opts *options) error {
	switch {
	case arg == "--help":
		fmt.Print(helpText)
		os.Exit(0)
	case arg == "--version":
		fmt.Println(versionText)
		os.Exit(0)
	case arg == "-e" || arg == "--canonicalize-existing":
		opts.mode = modeExisting
	case arg == "-m" || arg == "--canonicalize-missing":
		opts.mode = modeMissing
	case arg == "-s" || arg == "--strip" || arg == "--no-symlinks":
		opts.mode = modeStrip
	case arg == "-z" || arg == "--zero":
		opts.zero = true
	case strings.HasPrefix(arg, "--relative-to="):
		opts.relativeTo = strings.TrimPrefix(arg, "--relative-to=")
	case strings.HasPrefix(arg, "--relative-base="):
		opts.relativeBase = strings.TrimPrefix(arg, "--relative-base=")
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}
