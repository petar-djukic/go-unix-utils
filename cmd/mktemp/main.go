// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mktemp implements GNU mktemp: create temporary files or directories.
//
// Implements prd036-mktemp R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3,
// R3.1, R3.2, R3.3, R3.4, R3.5, R3.6, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName        = "mktemp"
	defaultTemplate = "tmp.XXXXXXXXXX"
	minXCount       = 3
	maxAttempts     = 100
	randChars       = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

// options holds parsed command-line flags for mktemp.
type options struct {
	dirMode    bool
	parentDir  string
	hasParent  bool
	suffix     string
	tMode      bool // R3.4: -t legacy BSD compatibility mode
	dryRun     bool // R3.5: -u/--dry-run mode
	quiet      bool // R3.6: -q/--quiet suppress errors
	positional []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes mktemp logic.
// R1.5: exits 0 on success, 1 on failure with error to stderr.
// R3.6: -q suppresses error messages on failure.
func run(args []string, stdout, stderr *os.File) int {
	opts, err := parseArgs(args)
	if err != nil {
		printErrQuiet(stderr, err, opts.quiet)
		return 1
	}
	tmpl, err := resolveTemplate(opts)
	if err != nil {
		printErrQuiet(stderr, err, opts.quiet)
		return 1
	}
	path, err := createTemp(tmpl, opts.suffix, opts.dirMode, opts.dryRun)
	if err != nil {
		printErrQuiet(stderr, err, opts.quiet)
		return 1
	}
	// R3.5: print dry-run warning to stderr
	if opts.dryRun {
		fmt.Fprintf(stderr, "%s: warning: remember to create the file/directory\n", progName) //nolint:errcheck
	}
	fmt.Fprintln(stdout, path) //nolint:errcheck
	return 0
}

// printErr writes a formatted error message to stderr.
func printErr(w *os.File, err error) {
	fmt.Fprintf(w, "%s: %s\n", progName, err) //nolint:errcheck
}

// printErrQuiet writes a formatted error message unless quiet mode is active.
// R3.6: -q/--quiet suppresses error messages on failure.
func printErrQuiet(w *os.File, err error, quiet bool) {
	if !quiet {
		printErr(w, err)
	}
}

// parseArgs extracts options and positional arguments from args.
// R2.1: -d/--directory flag.
// R3.1: -p DIR / --tmpdir=DIR flag.
// R3.2: --tmpdir (no value) flag.
// R3.3: --suffix=SUFF flag.
// R3.4: -t legacy mode flag.
// R3.5: -u/--dry-run flag.
// R3.6: -q/--quiet flag.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			opts.positional = append(opts.positional, args[i+1:]...)
			break
		}
		consumed, err := parseFlag(&opts, args, &i)
		if err != nil {
			return opts, err
		}
		if !consumed {
			opts.positional = append(opts.positional, a)
		}
	}
	return opts, nil
}

// parseFlag attempts to parse a single flag from args[*idx].
// Returns true if the argument was consumed as a flag.
func parseFlag(opts *options, args []string, idx *int) (bool, error) {
	a := args[*idx]
	switch {
	case a == "-d" || a == "--directory":
		opts.dirMode = true
	case a == "-p":
		*idx++
		if *idx >= len(args) {
			return false, fmt.Errorf("option '-p' requires an argument")
		}
		opts.parentDir = args[*idx]
		opts.hasParent = true
	case strings.HasPrefix(a, "--tmpdir="):
		// R3.1: --tmpdir=DIR uses DIR as parent directory.
		opts.parentDir = a[len("--tmpdir="):]
		opts.hasParent = true
	case a == "--tmpdir":
		// R3.2: --tmpdir without value uses TMPDIR or /tmp.
		opts.hasParent = true
	case strings.HasPrefix(a, "--suffix="):
		// R3.3: --suffix=SUFF appended after random characters.
		opts.suffix = a[len("--suffix="):]
	case a == "-t":
		// R3.4: legacy BSD compatibility mode.
		opts.tMode = true
	case a == "-u" || a == "--dry-run":
		// R3.5: dry-run mode.
		opts.dryRun = true
	case a == "-q" || a == "--quiet":
		// R3.6: suppress error messages.
		opts.quiet = true
	default:
		return false, nil
	}
	return true, nil
}

// resolveTemplate determines the full template path from parsed options.
// R1.1: default directory is TMPDIR or /tmp when no template given.
// R1.2: default template is tmp.XXXXXXXXXX.
// R3.1: -p DIR overrides the parent directory.
// R3.2: --tmpdir uses TMPDIR or /tmp as parent.
// R3.4: -t treats template as filename in TMPDIR.
func resolveTemplate(opts options) (string, error) {
	if len(opts.positional) > 1 {
		return "", fmt.Errorf("too many templates")
	}
	tmpl := defaultTemplate
	if len(opts.positional) == 1 {
		tmpl = opts.positional[0]
	}
	// R3.4: -t forces template into TMPDIR directory.
	if opts.tMode {
		return resolveTMode(tmpl)
	}
	dir := resolveDir(opts, len(opts.positional) == 1)
	if dir != "" {
		tmpl = filepath.Join(dir, tmpl)
	}
	return tmpl, nil
}

// resolveTMode handles -t flag: treats template as a filename in TMPDIR.
// R3.4: the template must not contain directory separators.
func resolveTMode(tmpl string) (string, error) {
	if strings.Contains(tmpl, "/") {
		return "", fmt.Errorf("invalid template, %q, contains directory separator", tmpl)
	}
	return filepath.Join(defaultDir(), tmpl), nil
}

// resolveDir determines the parent directory based on options.
// When -p or --tmpdir is set, uses the specified or default directory.
// When no parent flag is set and no explicit template, uses defaultDir.
func resolveDir(opts options, hasExplicitTemplate bool) string {
	if opts.hasParent {
		if opts.parentDir != "" {
			return opts.parentDir
		}
		return defaultDir()
	}
	if !hasExplicitTemplate {
		return defaultDir()
	}
	return ""
}

// defaultDir returns TMPDIR if set, or /tmp as fallback.
// R1.1: directory selection logic.
func defaultDir() string {
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	return "/tmp"
}

// createTemp creates a temporary file or directory from a template path.
// R1.3: replaces trailing X characters with random alphanumeric characters.
// R3.3: appends suffix after random characters.
// R3.5: in dry-run mode, generates a name without creating.
func createTemp(tmpl, suffix string, dirMode, dryRun bool) (string, error) {
	if err := validateSuffix(suffix); err != nil {
		return "", err
	}
	xCount := countTrailingX(filepath.Base(tmpl))
	if xCount < minXCount {
		return "", fmt.Errorf("too few X's in template '%s'", filepath.Base(tmpl))
	}
	prefix := tmpl[:len(tmpl)-xCount]
	if dryRun {
		return generateName(prefix, xCount, suffix)
	}
	return tryCreate(prefix, xCount, suffix, tmpl, dirMode)
}

// generateName produces a random name without creating anything.
// R3.5: -u/--dry-run returns the generated path.
func generateName(prefix string, xCount int, suffix string) (string, error) {
	rs, err := randomString(xCount)
	if err != nil {
		return "", err
	}
	return prefix + rs + suffix, nil
}

// validateSuffix checks that the suffix does not contain a directory separator.
// R3.3: slash after X sequence is not allowed with --suffix.
func validateSuffix(suffix string) error {
	if strings.Contains(suffix, "/") {
		return fmt.Errorf("invalid suffix '%s', contains directory separator", suffix)
	}
	return nil
}

// tryCreate attempts to create a unique file or directory with random
// characters, retrying on name collisions.
func tryCreate(prefix string, xCount int, suffix, tmpl string, dirMode bool) (string, error) {
	kind := "file"
	if dirMode {
		kind = "directory"
	}
	for range maxAttempts {
		path, err := attemptCreate(prefix, xCount, suffix, dirMode)
		if err == nil {
			return path, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("failed to create %s via template '%s': %v", kind, tmpl, err)
		}
	}
	return "", fmt.Errorf("failed to create %s via template '%s': too many collisions", kind, tmpl)
}

// attemptCreate generates a random name and tries to create the file
// or directory atomically.
// R1.4: file permission mode 0600.
// R2.2: directory permission mode 0700.
func attemptCreate(prefix string, xCount int, suffix string, dirMode bool) (string, error) {
	rs, err := randomString(xCount)
	if err != nil {
		return "", err
	}
	path := prefix + rs + suffix
	if dirMode {
		return path, os.Mkdir(path, 0o700)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	f.Close() // best-effort close; file already created
	return path, nil
}

// countTrailingX counts consecutive 'X' characters at the end of s.
func countTrailingX(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != 'X' {
			break
		}
		n++
	}
	return n
}

// randomString generates a string of n random alphanumeric characters
// using crypto/rand for secure randomness.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(randChars)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = randChars[idx.Int64()]
	}
	return string(b), nil
}
