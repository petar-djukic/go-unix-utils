// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp: Create Temporary Files or Directories.
// Covers R1.1-R1.5 (default behavior), R2.1-R2.3 (directory mode),
// R3.1-R3.4 (template and location control).
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
	directory bool   // R2.1: create directory instead of file.
	parentDir string // R3.1: -p DIR or --tmpdir=DIR override.
	suffix    string // R3.3: --suffix=SUFF appended after random chars.
	tFlag     bool   // R3.4: -t legacy BSD mode.
	useTmpdir bool   // R3.2: --tmpdir without value.
}

// run validates the template and creates the temporary file or directory.
// R1.5: exits 0 on success, 1 on failure with stderr diagnostic.
func run(cfg config, template string) int {
	// R3.3: suffix must not contain directory separator.
	if err := validateSuffix(cfg.suffix); err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		return 1
	}

	trailingXs := countTrailingXs(template)
	if trailingXs < minTrailingXs {
		fmt.Fprintf(os.Stderr,
			"mktemp: too few X's in template '%s'\n", template)
		return 1
	}

	dir, base := resolveTemplate(cfg, template)
	path, err := createTemp(cfg, dir, base, trailingXs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		return 1
	}

	// R1.3, R2.3: print the path of the created file or directory to stdout.
	fmt.Println(path)
	return 0
}

// validateSuffix checks that the suffix doesn't contain directory separators.
// R3.3: suffix must not contain '/'.
func validateSuffix(suffix string) error {
	if strings.Contains(suffix, "/") {
		return fmt.Errorf(
			"invalid suffix '%s', contains directory separator", suffix)
	}
	return nil
}

// resolveTemplate determines the parent directory and base name.
// R3.1: -p DIR overrides TMPDIR.
// R3.2: --tmpdir without value uses TMPDIR or /tmp.
// R3.4: -t forces template into TMPDIR (or -p dir).
func resolveTemplate(cfg config, template string) (string, string) {
	if cfg.parentDir != "" {
		return cfg.parentDir, filepath.Base(template)
	}
	if cfg.tFlag || cfg.useTmpdir {
		return effectiveTmpDir(), filepath.Base(template)
	}
	return splitTemplate(template)
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
		randPart, err := randomString(xs)
		if err != nil {
			return "", fmt.Errorf("generating random name: %w", err)
		}
		path := filepath.Join(dir, prefix+randPart+cfg.suffix)
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
// R2.1: -d creates directory; R2.2: directory mode 0700.
func createEntry(cfg config, path string) error {
	if cfg.directory {
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

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--":
			return handleDoubleDash(cfg, args[i+1:])
		case arg == "--help":
			return config{}, "", printHelp()
		case arg == "--version":
			return config{}, "", printVersion()
		case strings.HasPrefix(arg, "--"):
			exit = parseLongFlag(arg, &cfg)
			if exit >= 0 {
				return config{}, "", exit
			}
		case isShortFlags(arg):
			exit = parseShortFlags(arg, &cfg, args, &i)
			if exit >= 0 {
				return
			}
		default:
			if templateSet {
				fmt.Fprintln(os.Stderr, "mktemp: too many templates")
				return config{}, "", 1
			}
			template = arg
			templateSet = true
		}
		i++
	}
	return
}

// parseLongFlag handles long-form options (--directory, --tmpdir, --suffix).
func parseLongFlag(arg string, cfg *config) int {
	switch {
	case arg == "--directory":
		cfg.directory = true
	case arg == "--tmpdir":
		// R3.2: --tmpdir without value uses TMPDIR or /tmp.
		cfg.useTmpdir = true
	case strings.HasPrefix(arg, "--tmpdir="):
		// R3.1: --tmpdir=DIR uses DIR as parent directory.
		cfg.parentDir = arg[len("--tmpdir="):]
	case strings.HasPrefix(arg, "--suffix="):
		// R3.3: --suffix=SUFF appends SUFF after random chars.
		cfg.suffix = arg[len("--suffix="):]
	default:
		fmt.Fprintf(os.Stderr,
			"mktemp: unrecognized option '%s'\n", arg)
		return 1
	}
	return -1
}

// handleDoubleDash processes arguments after "--".
func handleDoubleDash(cfg config, rest []string) (config, string, int) {
	tmpl := defaultTemplate
	if len(rest) > 0 {
		tmpl = rest[0]
	}
	if len(rest) > 1 {
		fmt.Fprintln(os.Stderr, "mktemp: too many templates")
		return config{}, "", 1
	}
	return cfg, tmpl, -1
}

// isShortFlags returns true if the argument looks like a short flag group.
func isShortFlags(arg string) bool {
	return strings.HasPrefix(arg, "-") && len(arg) > 1 &&
		!strings.HasPrefix(arg, "--")
}

// parseShortFlags handles combined single-char flags like -d, -t, -p.
// Returns -1 to continue, >= 0 for early exit.
func parseShortFlags(arg string, cfg *config, args []string, idx *int) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'd':
			cfg.directory = true
		case 't':
			// R3.4: -t treats template as filename in TMPDIR.
			cfg.tFlag = true
		case 'p':
			return parsePFlag(arg[j+1:], cfg, args, idx)
		default:
			fmt.Fprintf(os.Stderr,
				"mktemp: invalid option -- '%c'\n", arg[j])
			return 1
		}
	}
	return -1
}

// parsePFlag handles the -p flag which takes a directory argument.
// R3.1: -p DIR uses DIR as the parent directory.
func parsePFlag(rest string, cfg *config, args []string, idx *int) int {
	if rest != "" {
		cfg.parentDir = rest
		return -1
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintln(os.Stderr,
			"mktemp: option requires an argument -- 'p'")
		return 1
	}
	cfg.parentDir = args[*idx]
	return -1
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mktemp [OPTION]... [TEMPLATE]
Create a temporary file or directory, safely, and print its name.
TEMPLATEs must contain at least 3 consecutive 'X's in last component.
If TEMPLATE is not specified, use tmp.XXXXXXXXXX.

  -d, --directory     create a directory, not a file
  -p DIR, --tmpdir=DIR  use DIR as the parent directory
      --tmpdir        use $TMPDIR or /tmp as parent directory
      --suffix=SUFF   append SUFF to TEMPLATE
  -t                  interpret TEMPLATE as a single file name component

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
