// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mktemp: create temporary files or directories.
// Implements srd036 R1.1-R1.5, R2.1-R2.3, R3.1-R3.6.
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

const programName = "mktemp"

// defaultTemplate is used when no TEMPLATE argument is provided.
// R1.2: tmp.XXXXXXXXXX with 10 random characters.
const defaultTemplate = "tmp.XXXXXXXXXX"

// minXCount is the minimum number of trailing X characters required.
// R1.3: template must contain at least 3 consecutive trailing X characters.
const minXCount = 3

// randChars is the alphanumeric character set for X replacement.
const randChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// maxAttempts is the retry limit for name collision avoidance.
const maxAttempts = 100

// usageText is the --help output printed to stdout.
const usageText = `Usage: mktemp [OPTION]... [TEMPLATE]
Create a temporary file or directory, safely, and print its name.
TEMPLATE must contain at least 3 consecutive 'X's in last component.
If TEMPLATE is not specified, use tmp.XXXXXXXXXX, and --tmpdir is implied.

  -d, --directory     create a directory, not a file
  -u, --dry-run       do not create anything; merely print a name (unsafe)
  -q, --quiet         suppress diagnostics about file/dir-creation failure
      --suffix=SUFF   append SUFF to TEMPLATE; SUFF must not contain a slash
  -p DIR, --tmpdir[=DIR]  interpret TEMPLATE relative to DIR; if DIR is not
                        specified, use $TMPDIR if set, else /tmp.  With
                        this option, TEMPLATE must not be an absolute name;
                        unlike with -t, TEMPLATE may contain slashes, but
                        mktemp creates only the final component
  -t                  interpret TEMPLATE as a single file name component,
                        relative to a directory: $TMPDIR, if set; else the
                        directory specified via -p; else /tmp [deprecated]
      --help          display this help and exit
      --version       output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "mktemp (go-unix-utils) 0.1.0\n"

// config holds parsed command-line options for mktemp.
type config struct {
	directory bool   // -d, --directory
	dryRun    bool   // -u, --dry-run
	quiet     bool   // -q, --quiet
	tmpdir    string // -p DIR, --tmpdir[=DIR]
	tmpdirSet bool   // whether --tmpdir/-p was explicitly provided
	suffix    string // --suffix=SUFF
	legacyT   bool   // -t (legacy BSD compatibility)
	help      bool   // --help
	version   bool   // --version
	template  string // positional TEMPLATE argument
}

// R1.5: main entry with SIGPIPE handler and flag parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\nTry '%s --help' for more information.\n",
			programName, err, programName)
		os.Exit(1)
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

// run executes the mktemp logic and returns the exit code.
// R1.5: returns 0 on success, 1 on failure.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	// R3.5: warn that -u/--dry-run is discouraged.
	if cfg.dryRun {
		fmt.Fprintf(os.Stderr, "%s: warning: remember to delete the file when you are done\n", programName)
	}

	if err := validateTemplate(cfg.template, cfg.suffix); err != nil {
		if !cfg.quiet {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		}
		return 1
	}

	path, err := createTemp(cfg)
	if err != nil {
		if !cfg.quiet {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		}
		return 1
	}

	// R2.3: print the resulting path to stdout followed by a newline.
	fmt.Fprintln(os.Stdout, path)
	return 0
}

// createTemp resolves the directory, expands the template, and creates
// the temporary file or directory.
func createTemp(cfg config) (string, error) {
	dir := resolveDir(cfg)
	base := filepath.Base(cfg.template)

	for range maxAttempts {
		name, err := expandXs(base)
		if err != nil {
			return "", fmt.Errorf("failed to generate random name: %w", err)
		}
		path := filepath.Join(dir, name+cfg.suffix)

		if cfg.dryRun {
			return path, nil
		}

		err = createEntry(path, cfg.directory)
		if err == nil {
			return path, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("too many attempts; giving up")
}

// createEntry creates a file (mode 0600) or directory (mode 0700) at path.
// R1.4: file mode 0600. R2.1/R2.2: directory mode 0700.
// R3.6: returns wrapped errors for diagnostic messages.
func createEntry(path string, isDir bool) error {
	if isDir {
		if err := os.Mkdir(path, 0o700); err != nil {
			return fmt.Errorf("failed to create directory via template '%s': %w", path, err)
		}
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create file via template '%s': %w", path, err)
	}
	return f.Close()
}

// resolveDir determines the parent directory for temp entry creation.
func resolveDir(cfg config) string {
	// R3.4: -t uses TMPDIR first, then -p dir, then /tmp.
	if cfg.legacyT {
		return resolveLegacyT(cfg)
	}

	// R3.1: -p DIR or --tmpdir=DIR overrides TMPDIR.
	if cfg.tmpdirSet && cfg.tmpdir != "" {
		return cfg.tmpdir
	}

	// If template has a directory component, use it.
	dir := filepath.Dir(cfg.template)
	if dir != "." {
		return dir
	}

	// R3.2: --tmpdir without value uses TMPDIR or /tmp.
	if cfg.tmpdirSet {
		return envTmpDir()
	}

	// R1.1: default template implies --tmpdir (TMPDIR or /tmp).
	if cfg.template == defaultTemplate {
		return envTmpDir()
	}

	// Custom template with no dir flags: current directory.
	return "."
}

// resolveLegacyT resolves the directory for -t mode.
// R3.4: precedence is TMPDIR > -p dir > /tmp.
func resolveLegacyT(cfg config) string {
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	if cfg.tmpdirSet && cfg.tmpdir != "" {
		return cfg.tmpdir
	}
	return "/tmp"
}

// envTmpDir returns TMPDIR if set, otherwise /tmp.
func envTmpDir() string {
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	return "/tmp"
}

// expandXs replaces trailing X characters in tmpl with random alphanumeric
// characters. R1.3/R2.2: X-suffix expansion.
func expandXs(tmpl string) (string, error) {
	xCount := countTrailingX(tmpl)
	prefix := tmpl[:len(tmpl)-xCount]

	suffix := make([]byte, xCount)
	max := big.NewInt(int64(len(randChars)))
	for i := range suffix {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		suffix[i] = randChars[n.Int64()]
	}
	return prefix + string(suffix), nil
}

// validateTemplate checks that the template contains at least minXCount
// consecutive trailing X characters, accounting for any --suffix.
// R1.3: minimum three consecutive trailing X characters required.
func validateTemplate(tmpl, suffix string) error {
	base := filepath.Base(tmpl)
	xCount := countTrailingX(base)
	if xCount < minXCount {
		return fmt.Errorf("too few X's in template '%s%s'", tmpl, suffix)
	}
	if suffix != "" && strings.Contains(suffix, "/") {
		return fmt.Errorf("invalid suffix '%s', contains directory separator", suffix)
	}
	return nil
}

// countTrailingX counts the number of consecutive 'X' characters at the
// end of the string.
func countTrailingX(s string) int {
	count := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != 'X' {
			break
		}
		count++
	}
	return count
}

// parseArgs parses command-line arguments into config.
// R1.2: defaults template to tmp.XXXXXXXXXX when none provided.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false
	templateSet := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			if templateSet {
				return config{}, fmt.Errorf("too many templates")
			}
			cfg.template = arg
			templateSet = true
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		skip, err := parseFlag(&cfg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}

	// R1.2: apply default template when none given.
	if !templateSet {
		cfg.template = defaultTemplate
	}

	return cfg, nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, args, idx)
	}
	return parseShortFlags(cfg, args, idx)
}

// parseLongFlag handles --name and --name=value flags.
func parseLongFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]

	if strings.HasPrefix(arg, "--suffix=") {
		cfg.suffix = arg[len("--suffix="):]
		return 0, nil
	}

	if strings.HasPrefix(arg, "--tmpdir=") {
		cfg.tmpdir = arg[len("--tmpdir="):]
		cfg.tmpdirSet = true
		return 0, nil
	}

	switch arg {
	case "--directory":
		cfg.directory = true
	case "--dry-run":
		cfg.dryRun = true
	case "--quiet":
		cfg.quiet = true
	case "--tmpdir":
		cfg.tmpdirSet = true
	case "--suffix":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--suffix' requires an argument")
		}
		cfg.suffix = args[idx+1]
		return 1, nil
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseShortFlags processes bundled short flags like -duq.
func parseShortFlags(cfg *config, args []string, idx int) (int, error) {
	flags := args[idx][1:]
	for i, ch := range flags {
		switch ch {
		case 'd':
			cfg.directory = true
		case 'u':
			cfg.dryRun = true
		case 'q':
			cfg.quiet = true
		case 't':
			cfg.legacyT = true
		case 'p':
			rest := flags[i+1:]
			if len(rest) > 0 {
				cfg.tmpdir = rest
				cfg.tmpdirSet = true
				return 0, nil
			}
			if idx+1 >= len(args) {
				return 0, fmt.Errorf("option requires an argument -- 'p'")
			}
			cfg.tmpdir = args[idx+1]
			cfg.tmpdirSet = true
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
