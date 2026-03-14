// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd036-mktemp R1.1–R1.5, R2.1–R2.3, R3.1–R3.6
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

// programName is the name used in error messages.
const programName = "mktemp"

// defaultTemplate is the template used when no template argument is provided.
//
// R1.2: Default template is tmp.XXXXXXXXXX.
const defaultTemplate = "tmp.XXXXXXXXXX"

// alphanumChars is the character set used to replace X placeholders.
const alphanumChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// minTrailingXs is the minimum number of trailing X characters required in a template.
//
// R3.3: Template must end with at least 3 consecutive X characters.
const minTrailingXs = 3

// maxRetries is the maximum number of creation attempts when a generated name
// collides with an existing file or directory. Matches GNU mktemp's TMP_MAX
// retry strategy.
//
// R3.4: Retry on collision.
const maxRetries = 238328

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Parse flags: -d/--directory, -u/--dry-run, -q/--quiet.
	dirMode := false
	dryRun := false
	quiet := false
	var remaining []string
	for _, arg := range args {
		switch arg {
		case "-d", "--directory":
			// R2.1: Directory mode.
			dirMode = true
		case "-u", "--dry-run":
			// R3.5: Dry-run mode.
			dryRun = true
		case "-q", "--quiet":
			// R3.6: Quiet mode.
			quiet = true
		case "--":
			// Stop processing flags.
			continue
		default:
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

	// R3.3: Validate template has at least 3 trailing X characters.
	if countTrailingXs(template) < minTrailingXs {
		if !quiet {
			fmt.Fprintf(os.Stderr, "%s: too few X's in template '%s'\n", programName, template)
		}
		os.Exit(1)
	}

	// R3.1: When template contains a directory separator, verify the parent
	// directory of the full path exists before attempting creation.
	fullTemplatePath := filepath.Join(dir, template)
	parentDir := filepath.Dir(fullTemplatePath)
	info, err := os.Stat(parentDir)
	if err != nil || !info.IsDir() {
		if !quiet {
			fmt.Fprintf(os.Stderr, "%s: failed to create %s via template '%s': No such file or directory\n",
				programName, entityType(dirMode), fullTemplatePath)
		}
		os.Exit(1)
	}

	// R3.5: Dry-run mode generates a name without creating the file or directory.
	if dryRun {
		fmt.Fprintf(os.Stderr, "%s: warning: remember to use --dry-run for safety\n", programName)
		name, err := expandTemplate(template)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "%s: failed to create %s via template '%s': %v\n",
					programName, entityType(dirMode), fullTemplatePath, err)
			}
			os.Exit(1)
		}
		path := filepath.Join(dir, name)
		fmt.Println(path)
		return
	}

	// R3.4: Retry loop for name collision (EEXIST).
	for range maxRetries {
		// R1.3: Replace trailing X characters with random alphanumeric characters.
		name, err := expandTemplate(template)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "%s: failed to create %s via template '%s': %v\n",
					programName, entityType(dirMode), fullTemplatePath, err)
			}
			os.Exit(1)
		}

		path := filepath.Join(dir, name)

		if dirMode {
			// R2.1, R2.2: Create directory with mode 0700 (owner read-write-execute only).
			err = os.Mkdir(path, 0o700)
		} else {
			// R1.4: Create the file with mode 0600 (owner read-write only).
			var f *os.File
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err == nil {
				_ = f.Close() // best-effort close; file was created successfully
			}
		}

		if err == nil {
			// R1.1, R2.3: Print the absolute path to stdout.
			fmt.Println(path)
			return
		}

		// R3.4: Retry on collision, fail on other errors.
		if os.IsExist(err) {
			continue
		}

		// R3.2: Permission denied and other creation errors.
		if !quiet {
			fmt.Fprintf(os.Stderr, "%s: failed to create %s via template '%s': %s\n",
				programName, entityType(dirMode), fullTemplatePath, unwrapPathError(err))
		}
		os.Exit(1)
	}

	// All retries exhausted.
	if !quiet {
		fmt.Fprintf(os.Stderr, "%s: failed to create %s via template '%s': too many attempts\n",
			programName, entityType(dirMode), fullTemplatePath)
	}
	os.Exit(1)
}

// entityType returns "directory" or "file" for error messages based on dirMode.
func entityType(dirMode bool) string {
	if dirMode {
		return "directory"
	}
	return "file"
}

// countTrailingXs returns the number of consecutive X characters at the end of s.
func countTrailingXs(s string) int {
	count := 0
	for i := len(s) - 1; i >= 0 && s[i] == 'X'; i-- {
		count++
	}
	return count
}

// unwrapPathError extracts the underlying error message from an os.PathError
// to avoid duplicating the path in error output.
func unwrapPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// expandTemplate replaces the trailing sequence of X characters in template
// with random alphanumeric characters using crypto/rand.
//
// R1.3: Consecutive trailing X characters are replaced with random characters.
func expandTemplate(template string) (string, error) {
	xCount := countTrailingXs(template)
	if xCount < minTrailingXs {
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
