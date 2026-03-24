// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp: Create Temporary Files or Directories.
// Covers R1.1-R1.5 (default behavior, template expansion, path output,
// trailing X validation, exit codes), R2.1 (directory mode with -d).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const (
	// R1.2: default template when no argument is provided.
	defaultTemplate = "tmp.XXXXXXXXXX"
	// R1.4: minimum trailing X characters required in template.
	minTrailingXs = 3
	// D4: alphanumeric charset for random replacement.
	randCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	// maxAttempts limits retries on name collision.
	maxAttempts = 100
)

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, tmpl, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg, tmpl))
}

// config holds parsed flag state.
type config struct {
	directory bool // R2.1: create directory instead of file.
}

// run validates the template and creates the temporary file or directory.
// R1.5: exits 0 on success, 1 on failure with stderr diagnostic.
func run(cfg config, template string) int {
	trailingXs := countTrailingXs(template)
	if trailingXs < minTrailingXs {
		fmt.Fprintf(os.Stderr,
			"mktemp: too few X's in template '%s'\n", template)
		return 1
	}

	dir, base := splitTemplate(template)
	path, err := createTemp(cfg, dir, base, trailingXs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		return 1
	}

	// R1.3: print the path of the created file or directory to stdout.
	fmt.Println(path)
	return 0
}

// splitTemplate separates the template into directory and base name.
// R1.1: if template has no directory component, uses TMPDIR or /tmp.
func splitTemplate(template string) (string, string) {
	if strings.Contains(template, "/") {
		return filepath.Dir(template), filepath.Base(template)
	}
	return effectiveTmpDir(), template
}

// effectiveTmpDir returns TMPDIR if set, otherwise /tmp.
// R1.1: TMPDIR overrides the default /tmp directory.
func effectiveTmpDir() string {
	if dir := os.Getenv("TMPDIR"); dir != "" {
		return dir
	}
	return "/tmp"
}

// countTrailingXs returns the number of consecutive X characters at
// the end of the template string.
func countTrailingXs(template string) int {
	count := 0
	for i := len(template) - 1; i >= 0 && template[i] == 'X'; i-- {
		count++
	}
	return count
}

// createTemp generates a unique name and creates the file or directory.
// Retries with new random names on collision up to maxAttempts.
func createTemp(cfg config, dir, base string, xs int) (string, error) {
	prefix := base[:len(base)-xs]
	for range maxAttempts {
		suffix, err := randomString(xs)
		if err != nil {
			return "", fmt.Errorf("generating random name: %w", err)
		}
		path := filepath.Join(dir, prefix+suffix)
		err = createEntry(cfg, path)
		if err == nil {
			return path, nil
		}
		if !os.IsExist(err) {
			return "", formatCreateError(cfg, path, err)
		}
	}
	return "", fmt.Errorf("exhausted %d name attempts", maxAttempts)
}

// createEntry creates a file or directory at the given path.
func createEntry(cfg config, path string) error {
	if cfg.directory {
		// R2.1, R2.2: create directory with mode 0700.
		return os.Mkdir(path, 0o700)
	}
	return createFile(path)
}

// createFile creates a file with exclusive mode to avoid races.
// R1.4: file permission mode 0600 (owner read-write only).
func createFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

// formatCreateError builds the GNU-style error message for creation failures.
func formatCreateError(cfg config, path string, err error) error {
	kind := "file"
	if cfg.directory {
		kind = "directory"
	}
	reason := extractReason(err)
	return fmt.Errorf("failed to create %s via template '%s': %s",
		kind, path, reason)
}

// extractReason unwraps a PathError to its OS-level reason string.
func extractReason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// randomString generates n random characters from randCharset.
// D4: uses crypto/rand for security matching GNU mktemp.
func randomString(n int) (string, error) {
	charsetLen := big.NewInt(int64(len(randCharset)))
	buf := make([]byte, n)
	for i := range buf {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		buf[i] = randCharset[idx.Int64()]
	}
	return string(buf), nil
}

// parseArgs processes command-line flags and returns configuration.
// exit is -1 to continue processing; >= 0 for early termination.
func parseArgs(args []string) (cfg config, template string, exit int) {
	exit = -1
	template = defaultTemplate
	templateSet := false

	for i, arg := range args {
		switch {
		case arg == "--":
			rest := args[i+1:]
			if len(rest) > 0 {
				template = rest[0]
			}
			if len(rest) > 1 {
				fmt.Fprintln(os.Stderr, "mktemp: too many templates")
				return config{}, "", 1
			}
			return
		case arg == "--help":
			return config{}, "", printHelp()
		case arg == "--version":
			return config{}, "", printVersion()
		case arg == "-d" || arg == "--directory":
			cfg.directory = true
		case isShortFlags(arg):
			exit = parseShortFlags(arg, &cfg)
			if exit >= 0 {
				return
			}
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr,
				"mktemp: unrecognized option '%s'\n", arg)
			return config{}, "", 1
		default:
			if templateSet {
				fmt.Fprintln(os.Stderr, "mktemp: too many templates")
				return config{}, "", 1
			}
			template = arg
			templateSet = true
		}
	}
	return
}

// isShortFlags returns true if the argument looks like a short flag group.
func isShortFlags(arg string) bool {
	return strings.HasPrefix(arg, "-") && len(arg) > 1 &&
		!strings.HasPrefix(arg, "--")
}

// parseShortFlags handles combined single-char flags like -d.
// Returns -1 to continue, >= 0 for early exit.
func parseShortFlags(arg string, cfg *config) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'd':
			cfg.directory = true
		default:
			fmt.Fprintf(os.Stderr,
				"mktemp: invalid option -- '%c'\n", arg[j])
			return 1
		}
	}
	return -1
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mktemp [OPTION]... [TEMPLATE]
Create a temporary file or directory, safely, and print its name.
TEMPLATEs must contain at least 3 consecutive 'X's in last component.
If TEMPLATE is not specified, use tmp.XXXXXXXXXX.

  -d, --directory  create a directory, not a file

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout,
		"mktemp (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
