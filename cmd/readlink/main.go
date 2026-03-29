// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/readlink implements GNU readlink: print symlink target or canonical path.
//
// Implements prd050-readlink R1.1, R1.2, R1.3, R1.4, R1.5, R1.6,
// R2.1, R2.2, R3.1, R3.2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "readlink"

// mode represents the canonicalization mode.
type mode int

const (
	// R1.1, R1.2: default — print immediate symlink target.
	modeDefault mode = iota
	// R1.3: -f, resolve full canonical path; last component may be missing.
	modeCanon
	// R1.4: -e, resolve full canonical path; every component must exist.
	modeStrict
	// R1.5: -m, resolve full canonical path; no component need exist.
	modeMissing
)

// config holds all parsed command-line options.
type config struct {
	mode      mode
	noNewline bool // R1.6: -n
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and resolves paths. Returns 0 on success, 1 on error.
func run(args []string, stdout, stderr *os.File) int {
	cfg, paths, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		printTryHelp(stderr)
		return 1
	}
	if len(paths) == 0 {
		printMissingOperand(stderr)
		return 1
	}

	// R2.2: silently ignore -n with multiple operands.
	if cfg.noNewline && len(paths) > 1 {
		cfg.noNewline = false
	}

	return processOperands(paths, cfg, stdout, stderr)
}

// processOperands resolves each operand and prints results.
func processOperands(paths []string, cfg config, stdout, stderr *os.File) int {
	exitCode := 0
	for _, p := range paths {
		result, resolveErr := resolve(p, cfg)
		if resolveErr != nil {
			printResolveError(stderr, cfg.mode, p, resolveErr)
			exitCode = 1
			continue
		}
		printResult(stdout, result, cfg, len(paths))
	}
	return exitCode
}

// printResolveError prints error to stderr for canonicalization modes.
// In default mode and -e mode, GNU readlink silently exits 1 on failure.
func printResolveError(stderr *os.File, m mode, p string, err error) {
	if m == modeDefault || m == modeStrict {
		return
	}
	fmt.Fprintf(stderr, "%s: %s: %s\n", progName, p, err) //nolint:errcheck
}

// printMissingOperand writes the missing-operand error to stderr.
func printMissingOperand(stderr *os.File) {
	fmt.Fprintf(stderr, "%s: missing operand\n", progName) //nolint:errcheck
	printTryHelp(stderr)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// printResult writes a resolved path to stdout, respecting -n.
func printResult(stdout *os.File, result string, cfg config, total int) {
	if cfg.noNewline && total == 1 {
		fmt.Fprint(stdout, result) //nolint:errcheck
	} else {
		fmt.Fprintln(stdout, result) //nolint:errcheck
	}
}

// resolve dispatches to the appropriate resolution function.
func resolve(p string, cfg config) (string, error) {
	switch cfg.mode {
	case modeCanon:
		return resolveCanon(p)
	case modeStrict:
		return resolveStrict(p)
	case modeMissing:
		return resolveMissing(p)
	default:
		return resolveDefault(p)
	}
}

// resolveDefault reads the immediate symlink target (R1.1, R1.2).
func resolveDefault(p string) (string, error) {
	target, err := os.Readlink(p)
	if err != nil {
		return "", err
	}
	return target, nil
}

// resolveCanon resolves -f: full canonical path, last may not exist.
func resolveCanon(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", errMessage(err)
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)

	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", errMessage(err)
	}
	full := filepath.Join(resolvedDir, base)
	if resolved, evalErr := filepath.EvalSymlinks(full); evalErr == nil {
		return resolved, nil
	}
	return full, nil
}

// resolveStrict resolves -e: every component must exist (R1.4).
func resolveStrict(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", errMessage(err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", errMessage(err)
	}
	return abs, nil
}

// resolveMissing resolves -m: no component need exist (R1.5).
func resolveMissing(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", errMessage(err)
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

// parseArgs extracts the config and path operands from arguments.
func parseArgs(args []string) (config, []string, error) {
	cfg := config{}
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
		if parsed, err := parseLong(arg, &cfg); parsed {
			if err != nil {
				return cfg, nil, err
			}
			continue
		}
		if err := parseShort(arg, &cfg); err != nil {
			return cfg, nil, err
		}
	}
	return cfg, paths, nil
}

// parseLong handles long flags. Returns true if the arg was a long flag.
func parseLong(arg string, cfg *config) (bool, error) {
	switch arg {
	case "--canonicalize":
		cfg.mode = modeCanon
		return true, nil
	case "--canonicalize-existing":
		cfg.mode = modeStrict
		return true, nil
	case "--canonicalize-missing":
		cfg.mode = modeMissing
		return true, nil
	case "--no-newline":
		cfg.noNewline = true
		return true, nil
	}
	if strings.HasPrefix(arg, "--") {
		return true, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return false, nil
}

// parseShort handles short flag bundles like -fn, -e.
func parseShort(arg string, cfg *config) error {
	for _, ch := range arg[1:] {
		switch ch {
		case 'f':
			cfg.mode = modeCanon
		case 'e':
			cfg.mode = modeStrict
		case 'm':
			cfg.mode = modeMissing
		case 'n':
			cfg.noNewline = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// errMessage extracts the inner message from a *os.PathError.
func errMessage(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
