// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mktemp: create temporary files or directories.
// Implements srd036 R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
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

// R1.1: main entry with SIGPIPE handler and flag parsing.
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
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	// R1.3: validate that template has at least minXCount trailing X characters.
	if err := validateTemplate(cfg.template, cfg.suffix); err != nil {
		if !cfg.quiet {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		}
		return 1
	}

	// TODO: R2, R3 — actual file/directory creation will be implemented
	// in subsequent tasks.
	return 0
}

// validateTemplate checks that the template contains at least minXCount
// consecutive trailing X characters, accounting for any --suffix.
// R1.3: minimum three consecutive trailing X characters required.
func validateTemplate(tmpl, suffix string) error {
	// The X sequence is at the end of the template, before any suffix.
	xCount := countTrailingX(tmpl)
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

	// Handle --suffix=VALUE form.
	if strings.HasPrefix(arg, "--suffix=") {
		cfg.suffix = arg[len("--suffix="):]
		return 0, nil
	}

	// Handle --tmpdir=VALUE form.
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
		// R3.2: --tmpdir without a value uses TMPDIR or /tmp.
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
			// -p requires a value: rest of this arg or next arg.
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
