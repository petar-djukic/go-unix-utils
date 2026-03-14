// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd036-mktemp R1.1–R1.5, R2.1–R2.3, R3.1–R3.6, R4.3, R4.4
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

	// Parse flags.
	dirMode := false
	dryRun := false
	quiet := false
	legacyT := false
	suffix := ""
	explicitDir := ""
	explicitDirSet := false
	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-d" || arg == "--directory":
			// R2.1: Directory mode.
			dirMode = true
		case arg == "-u" || arg == "--dry-run":
			// R3.5: Dry-run mode.
			dryRun = true
		case arg == "-q" || arg == "--quiet":
			// R3.6: Quiet mode.
			quiet = true
		case arg == "-t":
			// R3.4: Legacy BSD compatibility mode.
			legacyT = true
		case arg == "-p":
			// R3.1: -p DIR uses DIR as parent directory.
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'p'\n", programName)
				os.Exit(1)
			}
			i++
			explicitDir = args[i]
			explicitDirSet = true
		case strings.HasPrefix(arg, "--tmpdir="):
			// R3.1: --tmpdir=DIR uses DIR as parent directory.
			explicitDir = strings.TrimPrefix(arg, "--tmpdir=")
			explicitDirSet = true
		case arg == "--tmpdir":
			// R3.2: --tmpdir without value uses TMPDIR or /tmp.
			// No-op; default behavior already uses TMPDIR.
		case strings.HasPrefix(arg, "--suffix="):
			// R3.3: --suffix=SUFF appends SUFF after random characters.
			suffix = strings.TrimPrefix(arg, "--suffix=")
		case arg == "--":
			// Stop processing flags; remaining args are positional.
			remaining = append(remaining, args[i+1:]...)
			i = len(args)
		default:
			remaining = append(remaining, arg)
		}
	}

	// Determine template and whether a custom template was provided.
	template := defaultTemplate
	customTemplate := false
	if len(remaining) > 0 {
		// Use last non-flag argument as template.
		template = remaining[len(remaining)-1]
		customTemplate = true
	}

	// Determine parent directory.
	// GNU mktemp behavior: when no template is provided, --tmpdir is implied
	// (uses TMPDIR or /tmp). When a template IS provided without -t/-p/--tmpdir,
	// files are created in the current directory.
	var dir string
	if explicitDirSet {
		// R3.1: -p DIR or --tmpdir=DIR overrides TMPDIR.
		dir = explicitDir
	} else if legacyT || !customTemplate {
		// R3.4: -t forces TMPDIR. R1.1: No template implies --tmpdir.
		dir = os.Getenv("TMPDIR")
		if dir == "" {
			dir = "/tmp"
		}
	}

	// R3.3: Validate suffix does not contain directory separator.
	if suffix != "" && strings.Contains(suffix, "/") {
		if !quiet {
			fmt.Fprintf(os.Stderr, "%s: invalid suffix '%s', contains directory separator\n", programName, suffix)
		}
		os.Exit(1)
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
	fullTemplatePath := filepath.Join(dir, template+suffix)
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
		path := filepath.Join(dir, name+suffix)
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

		// R3.3: Append suffix after random character expansion.
		path := filepath.Join(dir, name+suffix)

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
