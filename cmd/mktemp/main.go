// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R1.1–R1.5, R2.1–R2.3: temporary file and directory
// creation with template validation, directory mode, and error handling.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName        = "mktemp"
	defaultTemplate = "tmp.XXXXXXXXXX"
	minXCount       = 3
	randCharset     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	maxAttempts     = 100
)

// config holds parsed command-line options for mktemp.
type config struct {
	dirMode   bool
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
	xCount := countTrailingXs(cfg.template)
	if xCount < minXCount {
		fmt.Fprintf(stderr, "%s: too few X's in template '%s'\n",
			progName, cfg.template)
		return 1
	}
	path, err := create(cfg, xCount)
	if err != nil {
		printError(stderr, cfg, err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}

// parseArgs extracts flags and the optional template from args.
// R2.1: -d/--directory enables directory mode.
func parseArgs(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var dirMode bool
	fs.BoolVar(&dirMode, "d", false, "create a directory, not a file")
	fs.BoolVar(&dirMode, "directory", false, "")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	template, dir := resolveTemplateAndDir(fs.Args())
	return config{
		dirMode:   dirMode,
		template:  template,
		parentDir: dir,
	}, nil
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

// create dispatches to file or directory creation based on config.
func create(cfg config, xCount int) (string, error) {
	if cfg.dirMode {
		return createTempDir(cfg.parentDir, cfg.template, xCount)
	}
	return createTempFile(cfg.parentDir, cfg.template, xCount)
}

// createTempFile creates a file with mode 0600 using O_EXCL for uniqueness.
// R1.2: replaces trailing X characters with random alphanumeric characters.
// R1.4: file permission mode is 0600 (owner read-write only).
func createTempFile(dir, template string, xCount int) (string, error) {
	prefix := template[:len(template)-xCount]
	for range maxAttempts {
		name, err := buildRandomName(prefix, xCount)
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
func createTempDir(dir, template string, xCount int) (string, error) {
	prefix := template[:len(template)-xCount]
	for range maxAttempts {
		name, err := buildRandomName(prefix, xCount)
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

// buildRandomName generates prefix + random alphanumeric suffix.
func buildRandomName(prefix string, xCount int) (string, error) {
	suffix, err := randomString(xCount)
	if err != nil {
		return "", fmt.Errorf("generating random name: %w", err)
	}
	return prefix + suffix, nil
}

// countTrailingXs returns the number of consecutive 'X' characters at the
// end of s.
func countTrailingXs(s string) int {
	count := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != 'X' {
			break
		}
		count++
	}
	return count
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
