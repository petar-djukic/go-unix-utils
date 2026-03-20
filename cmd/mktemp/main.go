// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R1.1–R1.4: core temporary file creation with
// default template, custom template, mode 0600, and SIGPIPE handling.
package main

import (
	"crypto/rand"
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
)

// R1.4: install SIGPIPE handler for GNU coreutils compatibility.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses the template argument and creates the temporary file.
// R1.1: default behavior creates in TMPDIR or /tmp.
// R1.3: custom template with minimum 3 trailing X characters.
func run(args []string, stdout, stderr io.Writer) int {
	template, dir := resolveTemplateAndDir(args)
	xCount := countTrailingXs(template)
	if xCount < minXCount {
		fmt.Fprintf(stderr, "%s: too few X's in template '%s'\n", progName, template)
		return 1
	}
	path, err := createTempFile(dir, template, xCount)
	if err != nil {
		printError(stderr, template, dir, err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
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

// createTempFile creates a file with mode 0600 using O_EXCL for uniqueness.
// R1.2: replaces trailing X characters with random alphanumeric characters.
// R1.4: file permission mode is 0600 (owner read-write only).
func createTempFile(dir, template string, xCount int) (string, error) {
	prefix := template[:len(template)-xCount]
	const maxAttempts = 100
	for range maxAttempts {
		suffix, err := randomString(xCount)
		if err != nil {
			return "", fmt.Errorf("generating random name: %w", err)
		}
		path := filepath.Join(dir, prefix+suffix)
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
// template and directory.
func printError(stderr io.Writer, template, dir string, err error) {
	fullTemplate := filepath.Join(dir, template)
	fmt.Fprintf(stderr, "%s: failed to create file via template '%s': %s\n",
		progName, fullTemplate, unwrapPathError(err))
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
