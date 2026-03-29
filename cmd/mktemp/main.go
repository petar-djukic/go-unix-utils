// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mktemp implements GNU mktemp: create temporary files or directories.
//
// Implements prd036-mktemp R1.1, R1.2, R1.3, R1.4.
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName        = "mktemp"
	defaultTemplate = "tmp.XXXXXXXXXX"
	minXCount       = 3
	maxAttempts     = 100
	randChars       = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes mktemp logic.
// R1.1: creates temp file in TMPDIR or /tmp by default.
// R1.5: exits 0 on success, 1 on failure.
func run(args []string, stdout, stderr *os.File) int {
	tmpl, err := resolveTemplate(args)
	if err != nil {
		printErr(stderr, err)
		return 1
	}
	path, err := createTempFile(tmpl)
	if err != nil {
		printErr(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, path) //nolint:errcheck
	return 0
}

// printErr writes a formatted error message to stderr.
func printErr(w *os.File, err error) {
	fmt.Fprintf(w, "%s: %s\n", progName, err) //nolint:errcheck
}

// resolveTemplate determines the full template path from arguments.
// R1.1: default directory is TMPDIR or /tmp when no template given.
// R1.2: default template is tmp.XXXXXXXXXX.
// R1.3: uses user-provided template as-is.
func resolveTemplate(args []string) (string, error) {
	positional := extractPositional(args)
	switch len(positional) {
	case 0:
		return filepath.Join(defaultDir(), defaultTemplate), nil
	case 1:
		return positional[0], nil
	default:
		return "", fmt.Errorf("too many templates")
	}
}

// extractPositional returns args after a -- separator, or all args if
// no separator is present.
func extractPositional(args []string) []string {
	for i, a := range args {
		if a == "--" {
			return args[i+1:]
		}
	}
	return args
}

// defaultDir returns TMPDIR if set, or /tmp as fallback.
// R1.1: directory selection logic.
func defaultDir() string {
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	return "/tmp"
}

// createTempFile creates a temporary file from a template path.
// R1.3: replaces trailing X characters with random alphanumeric characters.
// R1.4: creates with permission mode 0600.
func createTempFile(tmpl string) (string, error) {
	xCount := countTrailingX(filepath.Base(tmpl))
	if xCount < minXCount {
		return "", fmt.Errorf("too few X's in template '%s'", tmpl)
	}
	prefix := tmpl[:len(tmpl)-xCount]
	return tryCreate(prefix, xCount, tmpl)
}

// tryCreate attempts to create a unique file with random characters,
// retrying on name collisions.
func tryCreate(prefix string, xCount int, tmpl string) (string, error) {
	for range maxAttempts {
		path, err := attemptCreate(prefix, xCount)
		if err == nil {
			return path, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("failed to create file via template '%s': %v", tmpl, err)
		}
	}
	return "", fmt.Errorf("failed to create file via template '%s': too many collisions", tmpl)
}

// attemptCreate generates a random name and tries to create the file
// with mode 0600 using O_EXCL for atomicity.
// R1.4: permission mode 0600.
func attemptCreate(prefix string, xCount int) (string, error) {
	suffix, err := randomString(xCount)
	if err != nil {
		return "", err
	}
	path := prefix + suffix
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
