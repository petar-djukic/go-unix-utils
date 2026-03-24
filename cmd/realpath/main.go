// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/realpath implements prd049-realpath R1.1, R1.2, R1.3, R1.4, R1.5, R2.1.
// It prints the resolved absolute pathname for each argument.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "realpath"

// config holds the parsed command-line options.
type config struct {
	canonMissing bool
	noSymlinks   bool
	relativeTo   string
}

// R1.4: Install SIGPIPE handler at startup.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run parses flags, validates arguments, and resolves each path.
// R1.5: returns 0 if all paths resolve, 1 if any fail.
func run() int {
	cfg := parseFlags()
	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		return 1
	}
	exitCode := 0
	for _, arg := range flag.Args() {
		if err := processArg(arg, cfg); err != nil {
			printError(arg, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processArg resolves a single path and prints it to stdout.
func processArg(arg string, cfg config) error {
	resolved, err := resolve(arg, cfg)
	if err != nil {
		return err
	}
	if cfg.relativeTo != "" {
		resolved, err = makeRelative(resolved, cfg.relativeTo)
		if err != nil {
			return err
		}
	}
	fmt.Println(resolved)
	return nil
}

// parseFlags defines and parses all command-line flags.
func parseFlags() config {
	var cfg config
	var canonExisting bool

	// R1.3: -e / --canonicalize-existing (default behavior, accepted as no-op).
	flag.BoolVar(&canonExisting, "e", false, "")
	flag.BoolVar(&canonExisting, "canonicalize-existing", false, "")

	// R1.4: -m / --canonicalize-missing.
	flag.BoolVar(&cfg.canonMissing, "m", false, "")
	flag.BoolVar(&cfg.canonMissing, "canonicalize-missing", false, "")

	// R1.5: -s / --strip / --no-symlinks.
	flag.BoolVar(&cfg.noSymlinks, "s", false, "")
	flag.BoolVar(&cfg.noSymlinks, "strip", false, "")
	flag.BoolVar(&cfg.noSymlinks, "no-symlinks", false, "")

	// R2.1: --relative-to=DIR.
	flag.StringVar(&cfg.relativeTo, "relative-to", "", "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... FILE...\n", programName)
	}
	flag.Parse()
	return cfg
}

// resolve dispatches to the appropriate resolution strategy.
func resolve(path string, cfg config) (string, error) {
	switch {
	case cfg.canonMissing:
		return resolveMissing(path)
	case cfg.noSymlinks:
		return resolveNoSymlinks(path)
	default:
		return resolveCanonical(path)
	}
}

// resolveCanonical resolves symlinks and requires all components to exist.
// R1.1, R1.2, R1.3: filepath.Abs + filepath.EvalSymlinks.
func resolveCanonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveNoSymlinks cleans the path without resolving symlinks.
// R1.5: only clean . and .. and make absolute.
func resolveNoSymlinks(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// resolveMissing resolves existing components and constructs the rest.
// R1.4: no component needs to exist.
func resolveMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)
	resolvedDir, err := resolveMissing(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// makeRelative computes a relative path from relativeTo to the resolved path.
// R2.1: --relative-to=DIR output.
func makeRelative(resolved, relativeTo string) (string, error) {
	absRelTo, err := filepath.Abs(relativeTo)
	if err != nil {
		return "", err
	}
	return filepath.Rel(absRelTo, resolved)
}

// printError writes a GNU-format error message to stderr.
func printError(path string, err error) {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, err)
}
