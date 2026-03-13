// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd036-mktemp R1.1–R1.5, R2.1–R2.3
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "mktemp"

// defaultTemplate is the template used when no template argument is provided.
//
// R1.2: Default template is tmp.XXXXXXXXXX.
const defaultTemplate = "tmp.XXXXXXXXXX"

// alphanumChars is the character set used to replace X placeholders.
const alphanumChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.1: Parse -d/--directory flag.
	dirMode := false
	var remaining []string
	for _, arg := range args {
		if arg == "-d" || arg == "--directory" {
			dirMode = true
		} else if arg == "--" {
			// Stop processing flags.
			continue
		} else {
			remaining = append(remaining, arg)
		}
	}

	template := defaultTemplate
	if len(remaining) > 0 {
		// Use last non-flag argument as template.
		template = remaining[len(remaining)-1]
	}

	// R1.1: Use TMPDIR if set, otherwise /tmp.
	dir := os.Getenv("TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}

	// R1.3: Replace trailing X characters with random alphanumeric characters.
	name, err := expandTemplate(template)
	if err != nil {
		// R1.5: Print error to stderr and exit 1 on failure.
		fmt.Fprintf(os.Stderr, "%s: failed to create %s via template '%s': %v\n",
			programName, entityType(dirMode), template, err)
		os.Exit(1)
	}

	path := dir + "/" + name

	if dirMode {
		// R2.1, R2.2: Create directory with mode 0700 (owner read-write-execute only).
		if err := os.Mkdir(path, 0o700); err != nil {
			// R1.5: Print error to stderr and exit 1 on failure.
			fmt.Fprintf(os.Stderr, "%s: failed to create directory via template '%s': %v\n",
				programName, template, err)
			os.Exit(1)
		}
	} else {
		// R1.4: Create the file with mode 0600 (owner read-write only).
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			// R1.5: Print error to stderr and exit 1 on failure.
			fmt.Fprintf(os.Stderr, "%s: failed to create file via template '%s': %v\n",
				programName, template, err)
			os.Exit(1)
		}
		// best-effort close; file was created successfully
		_ = f.Close()
	}

	// R1.1, R2.3: Print the absolute path to stdout.
	// R1.5: Implicit exit 0 on success.
	fmt.Println(path)
}

// entityType returns "directory" or "file" for error messages based on dirMode.
func entityType(dirMode bool) string {
	if dirMode {
		return "directory"
	}
	return "file"
}

// expandTemplate replaces the trailing sequence of X characters in template
// with random alphanumeric characters using crypto/rand.
//
// R1.3: Consecutive trailing X characters are replaced with random characters.
func expandTemplate(template string) (string, error) {
	// Count trailing X characters.
	xCount := 0
	for i := len(template) - 1; i >= 0 && template[i] == 'X'; i-- {
		xCount++
	}
	if xCount < 3 {
		return "", fmt.Errorf("too few X's in template '%s'", template)
	}

	prefix := template[:len(template)-xCount]
	replacement, err := randomChars(xCount)
	if err != nil {
		return "", err
	}
	return prefix + replacement, nil
}

// randomChars generates n random characters from alphanumChars using crypto/rand.
//
// D3: Use crypto/rand for security.
func randomChars(n int) (string, error) {
	max := big.NewInt(int64(len(alphanumChars)))
	var b strings.Builder
	b.Grow(n)
	for range n {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("reading random: %w", err)
		}
		b.WriteByte(alphanumChars[idx.Int64()])
	}
	return b.String(), nil
}
