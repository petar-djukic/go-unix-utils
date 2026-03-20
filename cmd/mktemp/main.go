// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R1.1–R1.5, R2.1–R2.3, R3.1–R3.6, R4.3, R4.4:
// temporary file and directory creation with template validation, directory
// mode, suffix mode, quiet flag, --tmpdir/-p directory override, -t legacy
// mode, and -u/--dry-run.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
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
	randCharset     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	maxAttempts     = 100
	tmpdirPrefix    = "--tmpdir"
	dryRunWarning   = "warning: --dry-run is discouraged"
)

// config holds parsed command-line options for mktemp.
type config struct {
	dirMode   bool
	quiet     bool
	dryRun    bool
	suffix    string
	hasSuffix bool
	template  string
	parentDir string
}

// R1.5: install SIGPIPE handler for GNU coreutils compatibility.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses arguments, validates the template, and creates the temp
// file or directory. R1.5: exits 0 on success, 1 on failure.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return 1
	}
	prefix, xCount, suffix := parseTemplate(cfg.template)
	if cfg.hasSuffix {
		suffix = cfg.suffix
	}
	if err := validateTemplate(cfg.template, xCount, suffix); err != nil {
		if !cfg.quiet {
			fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		}
		return 1
	}
	if cfg.dryRun {
		return handleDryRun(cfg, prefix, xCount, suffix, stdout, stderr)
	}
	path, err := create(cfg, prefix, xCount, suffix)
	if err != nil {
		if !cfg.quiet {
			printError(stderr, cfg, err)
		}
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}

// handleDryRun generates and prints a path without creating the file or
// directory. R3.5: prints a warning that -u is discouraged.
func handleDryRun(cfg config, prefix string, xCount int, suffix string, stdout, stderr io.Writer) int {
	name, err := buildRandomName(prefix, xCount, suffix)
	if err != nil {
		if !cfg.quiet {
			printError(stderr, cfg, err)
		}
		return 1
	}
	path := filepath.Join(cfg.parentDir, name)
	fmt.Fprintf(stderr, "%s: %s\n", progName, dryRunWarning)
	fmt.Fprintln(stdout, path)
	return 0
}

// validateTemplate checks xCount >= minXCount and suffix has no slashes.
// R3.1, R3.3: validates template and suffix constraints.
func validateTemplate(template string, xCount int, suffix string) error {
	if xCount < minXCount {
		return fmt.Errorf("too few X's in template '%s'", template)
	}
	if strings.Contains(suffix, "/") {
		return fmt.Errorf("invalid suffix '%s', contains directory separator",
			suffix)
	}
	return nil
}

// parseArgs extracts flags and the optional template from args.
// R2.1: -d/--directory enables directory mode.
// R3.3: --suffix specifies an explicit suffix.
// R3.4: -t treats template as name in TMPDIR.
// R3.5: -u/--dry-run generates name without creating.
// R3.6: -q/--quiet suppresses error diagnostics.
func parseArgs(args []string, stderr io.Writer) (config, error) {
	args, tmpdirSet, tmpdirDir := prescanTmpdir(args)
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var dirMode, quiet, dryRun, tFlag bool
	var suffix, pDir string
	fs.BoolVar(&dirMode, "d", false, "create a directory, not a file")
	fs.BoolVar(&dirMode, "directory", false, "")
	fs.BoolVar(&quiet, "q", false, "suppress error messages")
	fs.BoolVar(&quiet, "quiet", false, "")
	fs.BoolVar(&dryRun, "u", false, "dry run, do not create")
	fs.BoolVar(&dryRun, "dry-run", false, "")
	fs.BoolVar(&tFlag, "t", false, "interpret template as name in TMPDIR")
	fs.StringVar(&suffix, "suffix", "", "append SUFF to template")
	fs.StringVar(&pDir, "p", "", "use DIR as parent directory")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	hasSuffix := flagWasSet(fs, "suffix")
	pDirSet := flagWasSet(fs, "p")
	if tFlag && !tmpdirSet && !pDirSet {
		tmpdirSet = true
	}
	template, dir := resolveLocation(
		fs.Args(), tmpdirSet, tmpdirDir, pDirSet, pDir,
	)
	return config{
		dirMode: dirMode, quiet: quiet, dryRun: dryRun,
		suffix: suffix, hasSuffix: hasSuffix,
		template: template, parentDir: dir,
	}, nil
}

// prescanTmpdir extracts --tmpdir[=DIR] from args before flag.Parse.
// Go's flag package cannot handle optional-value long flags natively.
// R3.5: --tmpdir without value uses TMPDIR or /tmp.
// R3.6: --tmpdir=DIR uses DIR as parent directory.
func prescanTmpdir(args []string) ([]string, bool, string) {
	var filtered []string
	var set bool
	var dir string
	for _, arg := range args {
		if arg == tmpdirPrefix {
			set = true
		} else if strings.HasPrefix(arg, tmpdirPrefix+"=") {
			set = true
			dir = arg[len(tmpdirPrefix+"="):]
		} else {
			filtered = append(filtered, arg)
		}
	}
	return filtered, set, dir
}

// flagWasSet returns whether a flag was explicitly provided on the command line.
func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// resolveLocation determines template and parent directory from positional
// args and flag overrides. R3.5: --tmpdir overrides parent directory.
// R3.1: -p DIR overrides parent directory.
func resolveLocation(posArgs []string, tmpdirSet bool, tmpdirDir string, pDirSet bool, pDir string) (string, string) {
	if tmpdirSet {
		return resolveTmpdirMode(posArgs, tmpdirDir)
	}
	if pDirSet {
		return resolveTmpdirMode(posArgs, pDir)
	}
	return resolveTemplateAndDir(posArgs)
}

// resolveTmpdirMode handles template resolution when --tmpdir or -p is active.
// The template argument is treated as a name pattern in the specified directory.
// An empty dir defaults to TMPDIR or /tmp.
func resolveTmpdirMode(posArgs []string, dir string) (string, string) {
	if dir == "" {
		dir = os.TempDir()
	}
	if len(posArgs) == 0 {
		return defaultTemplate, dir
	}
	return posArgs[0], dir
}

// resolveTemplateAndDir determines the template basename and parent directory.
// R1.1: no args uses TMPDIR (or /tmp) with the default template.
// R1.3: explicit template uses its directory component, or CWD if none.
func resolveTemplateAndDir(args []string) (string, string) {
	if len(args) == 0 {
		return defaultTemplate, os.TempDir()
	}
	template := args[0]
	dir := filepath.Dir(template)
	base := filepath.Base(template)
	return base, dir
}

// parseTemplate splits a template into prefix, X count, and suffix.
// R3.1: characters after the last X sequence are treated as a suffix.
// For "tmp.XXXXXX.txt" returns ("tmp.", 6, ".txt").
func parseTemplate(template string) (string, int, string) {
	lastX := strings.LastIndex(template, "X")
	if lastX == -1 {
		return template, 0, ""
	}
	suffix := template[lastX+1:]
	firstX := lastX
	for firstX > 0 && template[firstX-1] == 'X' {
		firstX--
	}
	xCount := lastX - firstX + 1
	prefix := template[:firstX]
	return prefix, xCount, suffix
}

// create dispatches to file or directory creation based on config.
func create(cfg config, prefix string, xCount int, suffix string) (string, error) {
	if cfg.dirMode {
		return createTempDir(cfg.parentDir, prefix, xCount, suffix)
	}
	return createTempFile(cfg.parentDir, prefix, xCount, suffix)
}

// createTempFile creates a file with mode 0600 using O_EXCL for uniqueness.
// R1.2: replaces X characters with random alphanumeric characters.
// R1.4: file permission mode is 0600 (owner read-write only).
func createTempFile(dir, prefix string, xCount int, suffix string) (string, error) {
	for range maxAttempts {
		name, err := buildRandomName(prefix, xCount, suffix)
		if err != nil {
			return "", err
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		f.Close() // best-effort close; file already created
		return path, nil
	}
	return "", fmt.Errorf("too many attempts to create unique name")
}

// createTempDir creates a directory with mode 0700 using Mkdir for uniqueness.
// R2.1: creates a directory instead of a file.
// R2.2: directory permission mode is 0700 (owner read-write-execute only).
func createTempDir(dir, prefix string, xCount int, suffix string) (string, error) {
	for range maxAttempts {
		name, err := buildRandomName(prefix, xCount, suffix)
		if err != nil {
			return "", err
		}
		path := filepath.Join(dir, name)
		err = os.Mkdir(path, 0o700)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("too many attempts to create unique name")
}

// buildRandomName generates prefix + random alphanumeric chars + suffix.
func buildRandomName(prefix string, xCount int, suffix string) (string, error) {
	randStr, err := randomString(xCount)
	if err != nil {
		return "", fmt.Errorf("generating random name: %w", err)
	}
	return prefix + randStr + suffix, nil
}

// randomString generates a string of n random alphanumeric characters
// using crypto/rand for security, matching GNU mktemp behavior.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	charsetLen := big.NewInt(int64(len(randCharset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		b[i] = randCharset[idx.Int64()]
	}
	return string(b), nil
}

// printError writes a failure message to stderr with context about the
// template and directory. R1.5: error printed to stderr on failure.
func printError(stderr io.Writer, cfg config, err error) {
	fullTemplate := filepath.Join(cfg.parentDir, cfg.template)
	kind := "file"
	if cfg.dirMode {
		kind = "directory"
	}
	fmt.Fprintf(stderr, "%s: failed to create %s via template '%s': %s\n",
		progName, kind, fullTemplate, unwrapPathError(err))
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
