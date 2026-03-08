// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the mktemp utility for creating temporary files
// or directories.
//
// Implements prd036-mktemp: default behavior (R1), directory mode (R2),
// template and location control (R3), differential testing (R4).
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

// randChars is the character set used for X replacement, matching GNU mktemp.
const randChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// defaultTemplate is the template used when no TEMPLATE argument is provided. R1.2.
const defaultTemplate = "tmp.XXXXXXXXXX"

// flags holds the parsed command-line options.
type flags struct {
	directory bool   // -d, --directory: create a directory instead of a file
	dryRun    bool   // -u, --dry-run: print name without creating
	quiet     bool   // -q, --quiet: suppress error messages
	suffix    string // --suffix=SUFF: append after random characters
	tmpdir    string // -p DIR, --tmpdir[=DIR]: parent directory
	useTmpdir bool   // whether --tmpdir was specified (even without value)
	legacyT   bool   // -t: interpret TEMPLATE relative to TMPDIR
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, template := parseArgs(os.Args[1:])

	if err := run(f, template); err != nil {
		if !f.quiet {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		os.Exit(1)
	}
}

// run executes the mktemp logic. Returns an error on failure.
func run(f flags, template string) error {
	// Determine the parent directory.
	dir := resolveDir(f)

	// Determine the template to use.
	if template == "" {
		template = defaultTemplate
	}

	// R3.4: -t treats template as filename prefix relative to TMPDIR.
	if f.legacyT {
		// Template is just a name component; place it in the resolved dir.
		if strings.Contains(template, "/") {
			return fmt.Errorf("mktemp: invalid template, '%s', contains directory separator", template)
		}
		template = filepath.Join(dir, template)
	} else if !filepath.IsAbs(template) && !strings.Contains(template, "/") {
		// No directory component — place in the resolved dir.
		template = filepath.Join(dir, template)
	}

	// Add suffix if specified. R3.3.
	if f.suffix != "" {
		// Validate: template must not contain slash after X sequence when --suffix is used.
		xIdx := findTrailingXs(template)
		afterX := template[xIdx:]
		if strings.Contains(afterX, "/") {
			return fmt.Errorf("mktemp: with --suffix, template must not contain directory separator after X's")
		}
		template = template + f.suffix
	}

	// Validate template has at least 3 consecutive trailing X's (before suffix).
	base := template
	if f.suffix != "" {
		base = strings.TrimSuffix(template, f.suffix)
	}
	xCount := countTrailingXs(base)
	if xCount < 3 {
		return fmt.Errorf("mktemp: too few X's in template '%s'", template)
	}

	// Generate the name.
	name, err := expandTemplate(template, f.suffix, xCount)
	if err != nil {
		return fmt.Errorf("mktemp: failed to create name from template: %w", err)
	}

	// R3.5: -u dry-run mode.
	if f.dryRun {
		fmt.Println(name)
		return nil
	}

	// Create file or directory.
	if f.directory {
		// R2.1, R2.2: create directory with mode 0700.
		if err := os.Mkdir(name, 0700); err != nil {
			return fmt.Errorf("mktemp: failed to create directory via template '%s': %s", template, unwrapErr(err))
		}
	} else {
		// R1.4: create file with mode 0600.
		fd, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("mktemp: failed to create file via template '%s': %s", template, unwrapErr(err))
		}
		fd.Close() //nolint:errcheck // best-effort close; file created successfully
	}

	fmt.Println(name)
	return nil
}

// resolveDir determines the parent directory for the temporary entry.
func resolveDir(f flags) string {
	// R3.1: -p DIR overrides TMPDIR.
	if f.tmpdir != "" {
		return f.tmpdir
	}
	// R3.2: --tmpdir without value, or default: use TMPDIR or /tmp.
	if dir := os.Getenv("TMPDIR"); dir != "" {
		return dir
	}
	return "/tmp"
}

// countTrailingXs returns the number of consecutive 'X' characters at the end of s.
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

// findTrailingXs returns the index just past the trailing X sequence.
func findTrailingXs(s string) int {
	return len(s) - countTrailingXs(s)
}

// expandTemplate replaces trailing X's (before suffix) with random characters.
func expandTemplate(template, suffix string, xCount int) (string, error) {
	base := template
	if suffix != "" {
		base = strings.TrimSuffix(template, suffix)
	}

	replacement, err := randomString(xCount)
	if err != nil {
		return "", err
	}

	prefix := base[:len(base)-xCount]
	return prefix + replacement + suffix, nil
}

// randomString generates a string of n random alphanumeric characters using crypto/rand.
func randomString(n int) (string, error) {
	max := big.NewInt(int64(len(randChars)))
	buf := make([]byte, n)
	for i := range n {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = randChars[idx.Int64()]
	}
	return string(buf), nil
}

// unwrapErr extracts the inner error message from an os.PathError.
func unwrapErr(err error) string {
	if pathErr, ok := err.(*os.PathError); ok {
		return pathErr.Err.Error()
	}
	return err.Error()
}

// parseArgs parses command-line arguments into flags and a template string.
func parseArgs(args []string) (flags, string) {
	var f flags
	var template string
	endFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endFlags || len(arg) == 0 || arg[0] != '-' {
			template = arg
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--directory":
				f.directory = true
			case arg == "--dry-run":
				f.dryRun = true
			case arg == "--quiet":
				f.quiet = true
			case arg == "--tmpdir":
				f.useTmpdir = true
				// --tmpdir without '=' means use default TMPDIR.
			case strings.HasPrefix(arg, "--tmpdir="):
				f.tmpdir = arg[len("--tmpdir="):]
				f.useTmpdir = true
			case strings.HasPrefix(arg, "--suffix="):
				f.suffix = arg[len("--suffix="):]
			case arg == "--suffix":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "mktemp: option '--suffix' requires an argument\n")
					os.Exit(1)
				}
				f.suffix = args[i]
			case arg == "--help":
				printHelp()
				os.Exit(0)
			case arg == "--version":
				printVersion()
				os.Exit(0)
			default:
				fmt.Fprintf(os.Stderr, "mktemp: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short options.
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'd':
				f.directory = true
			case 'u':
				f.dryRun = true
			case 'q':
				f.quiet = true
			case 't':
				f.legacyT = true
			case 'p':
				// -p DIR: next arg or rest of current arg.
				if j+1 < len(arg) {
					f.tmpdir = arg[j+1:]
				} else {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "mktemp: option requires an argument -- 'p'\n")
						os.Exit(1)
					}
					f.tmpdir = args[i]
				}
				j = len(arg) // break inner loop
			default:
				fmt.Fprintf(os.Stderr, "mktemp: invalid option -- '%c'\n", arg[j])
				os.Exit(1)
			}
		}
	}

	return f, template
}

// printHelp prints the usage message. R3 (--help).
func printHelp() {
	fmt.Print(`Usage: mktemp [OPTION]... [TEMPLATE]
Create a temporary file or directory, safely, and print its name.
TEMPLATE must contain at least 3 consecutive 'X's in last component.
If TEMPLATE is not specified, use tmp.XXXXXXXXXX, and --tmpdir is implied.

  -d, --directory     create a directory, not a file
  -u, --dry-run       do not create anything; merely print a name (unsafe)
  -q, --quiet         suppress diagnostics about file/dir-creation failure
      --suffix=SUFF   append SUFF to TEMPLATE; SUFF must not contain a slash
  -p DIR, --tmpdir[=DIR]  interpret TEMPLATE relative to DIR; if DIR is not
                       specified, use $TMPDIR if set, else /tmp
  -t                  interpret TEMPLATE as a single file name component,
                       relative to a directory: $TMPDIR, if set; else /tmp
      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion prints the version information. R3 (--version).
func printVersion() {
	fmt.Println("mktemp (go-unix-utils) 1.0")
}
