// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/realpath implements GNU realpath: print resolved absolute paths.
//
// Implements prd049-realpath R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
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

// config holds all parsed command-line options.
type config struct {
	mode         mode
	strip        bool   // R1.5: -s, --strip, --no-symlinks
	relativeTo   string // R2.1: --relative-to=DIR
	relativeBase string // R2.2: --relative-base=DIR
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
		return 1
	}
	if len(paths) == 0 {
		printMissingOperand(stderr)
		return 1
	}

	exitCode := 0
	for _, p := range paths {
		resolved, resolveErr := resolvePath(p, cfg)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, resolveErr) //nolint:errcheck
			exitCode = 1
			continue
		}
		output := applyRelative(resolved, cfg)
		fmt.Fprintln(stdout, output) //nolint:errcheck
	}
	return exitCode
}

// printMissingOperand writes the missing-operand error to stderr.
func printMissingOperand(stderr *os.File) {
	fmt.Fprintf(stderr, "%s: missing operand\n", progName)                   //nolint:errcheck
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// applyRelative adjusts the resolved path for --relative-to and --relative-base.
func applyRelative(resolved string, cfg config) string {
	hasTo := cfg.relativeTo != ""
	hasBase := cfg.relativeBase != ""

	if !hasTo && !hasBase {
		return resolved
	}
	// R2.3: both flags — apply relative-to only if path starts with base.
	if hasTo && hasBase {
		return applyBothRelative(resolved, cfg)
	}
	// R2.1: --relative-to only.
	if hasTo {
		return makeRelative(resolved, cfg.relativeTo)
	}
	// R2.2: --relative-base only.
	return applyRelativeBase(resolved, cfg.relativeBase)
}

// applyBothRelative handles R2.3: both --relative-to and --relative-base.
func applyBothRelative(resolved string, cfg config) string {
	if pathStartsWith(resolved, cfg.relativeBase) {
		return makeRelative(resolved, cfg.relativeTo)
	}
	return resolved
}

// applyRelativeBase handles R2.2: --relative-base only.
func applyRelativeBase(resolved, base string) string {
	if pathStartsWith(resolved, base) {
		return makeRelative(resolved, base)
	}
	return resolved
}

// makeRelative computes the relative path from base to target.
func makeRelative(target, base string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// pathStartsWith returns true if target starts with the prefix directory.
func pathStartsWith(target, prefix string) bool {
	// Normalize both to ensure trailing slashes don't interfere.
	cleanTarget := filepath.Clean(target)
	cleanPrefix := filepath.Clean(prefix)
	if cleanTarget == cleanPrefix {
		return true
	}
	return strings.HasPrefix(cleanTarget, cleanPrefix+string(filepath.Separator))
}

// parseArgs extracts the config and path operands from command-line arguments.
func parseArgs(args []string) (config, []string, error) {
	cfg := config{}
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
	// Resolve the relative dirs through the same canonicalization.
	if err := resolveRelativeDirs(&cfg); err != nil {
		return cfg, nil, err
	}
	return cfg, paths, nil
}

// resolveRelativeDirs resolves --relative-to and --relative-base dirs.
func resolveRelativeDirs(cfg *config) error {
	if cfg.relativeTo != "" {
		resolved, err := resolveDir(cfg.relativeTo, *cfg)
		if err != nil {
			return err
		}
		cfg.relativeTo = resolved
	}
	if cfg.relativeBase != "" {
		resolved, err := resolveDir(cfg.relativeBase, *cfg)
		if err != nil {
			return err
		}
		cfg.relativeBase = resolved
	}
	return nil
}

// resolveDir resolves a directory path using the current config's mode/strip.
func resolveDir(dir string, cfg config) (string, error) {
	return resolvePath(dir, cfg)
}

// parseLong handles long flags. Returns true if the arg was a long flag.
func parseLong(arg string, cfg *config) (bool, error) {
	switch arg {
	case "--canonicalize-existing":
		cfg.mode = modeStrict
		return true, nil
	case "--canonicalize-missing":
		cfg.mode = modeMissing
		return true, nil
	case "--strip", "--no-symlinks":
		cfg.strip = true
		return true, nil
	}
	// Handle --relative-to=DIR and --relative-base=DIR
	if val, ok := parseLongValue(arg, "--relative-to"); ok {
		cfg.relativeTo = val
		return true, nil
	}
	if val, ok := parseLongValue(arg, "--relative-base"); ok {
		cfg.relativeBase = val
		return true, nil
	}
	if strings.HasPrefix(arg, "--") {
		return true, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return false, nil
}

// parseLongValue extracts the value from --key=value. Returns ("", false) if
// the arg does not match the prefix.
func parseLongValue(arg, prefix string) (string, bool) {
	if !strings.HasPrefix(arg, prefix+"=") {
		return "", false
	}
	return arg[len(prefix)+1:], true
}

// parseShort handles short flag bundles like -em, -s.
func parseShort(arg string, cfg *config) error {
	for _, ch := range arg[1:] {
		switch ch {
		case 'e':
			cfg.mode = modeStrict
		case 'm':
			cfg.mode = modeMissing
		case 's':
			cfg.strip = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// resolvePath resolves a single path according to the given config.
func resolvePath(p string, cfg config) (string, error) {
	if cfg.strip {
		return resolveStrip(p, cfg.mode)
	}
	switch cfg.mode {
	case modeMissing:
		return resolveMissing(p)
	case modeStrict:
		return resolveStrict(p)
	default:
		return resolveDefault(p)
	}
}

// resolveStrip resolves a path without following symlinks (R1.5).
func resolveStrip(p string, m mode) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%s: %s", p, errMessage(err))
	}
	cleaned := filepath.Clean(abs)

	switch m {
	case modeStrict:
		if _, err := os.Lstat(cleaned); err != nil {
			return "", fmt.Errorf("%s: %s", p, errMessage(err))
		}
	case modeMissing:
		// No existence check needed.
	default:
		// Default: parent must exist.
		dir := filepath.Dir(cleaned)
		if _, err := os.Lstat(dir); err != nil {
			return "", fmt.Errorf("%s: %s", p, errMessage(err))
		}
	}
	return cleaned, nil
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
