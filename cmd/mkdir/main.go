// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mkdir: create directories.
// Implements srd034 R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "mkdir"

// usageText is the --help output printed to stdout.
const usageText = `Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask
  -p, --parents     no error if existing, make parent directories as needed
  -v, --verbose     print a message for each created directory
      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "mkdir (go-unix-utils) 0.1.0\n"

// config holds parsed command-line options for mkdir.
type config struct {
	parents bool   // -p, --parents
	mode    string // -m, --mode=MODE
	verbose bool   // -v, --verbose
	help    bool   // --help
	version bool   // --version
	dirs    []string
}

// R1.1: main entry with SIGPIPE handler and flag parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

// run executes the mkdir logic and returns the exit code.
// R1.2: processes each directory argument independently.
// R1.3, R1.4: prints error and continues on failure.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	if len(cfg.dirs) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}

	exitCode := 0
	for _, dir := range cfg.dirs {
		if err := createDir(cfg, dir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// createDir creates a single directory per the current config.
// R1.1: creates directory with os.Mkdir using default permissions.
// R1.3: returns error when directory already exists.
// R1.4: returns error when parent does not exist.
func createDir(cfg config, dir string) error {
	if cfg.parents {
		return createWithParents(cfg, dir)
	}
	if err := os.Mkdir(dir, 0o777); err != nil {
		return formatMkdirError(dir, err)
	}
	if cfg.verbose {
		fmt.Fprintf(os.Stdout, "%s: created directory '%s'\n", programName, dir)
	}
	return nil
}

// createWithParents creates a directory and its parents.
// Stub for future R2 implementation.
func createWithParents(cfg config, dir string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return formatMkdirError(dir, err)
	}
	if cfg.verbose {
		fmt.Fprintf(os.Stdout, "%s: created directory '%s'\n", programName, dir)
	}
	return nil
}

// formatMkdirError wraps a mkdir error to match GNU mkdir output format.
func formatMkdirError(dir string, err error) error {
	return fmt.Errorf("cannot create directory '%s': %s", dir, unwrapOSError(err))
}

// unwrapOSError extracts the underlying message from an *os.PathError.
func unwrapOSError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// parseArgs parses command-line arguments into config.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			cfg.dirs = append(cfg.dirs, arg)
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

	// Handle --mode=VALUE form.
	if strings.HasPrefix(arg, "--mode=") {
		cfg.mode = arg[len("--mode="):]
		return 0, nil
	}

	switch arg {
	case "--parents":
		cfg.parents = true
	case "--mode":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--mode' requires an argument")
		}
		cfg.mode = args[idx+1]
		return 1, nil
	case "--verbose":
		cfg.verbose = true
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseShortFlags processes bundled short flags like -pv.
func parseShortFlags(cfg *config, args []string, idx int) (int, error) {
	flags := args[idx][1:]
	for i, ch := range flags {
		switch ch {
		case 'p':
			cfg.parents = true
		case 'v':
			cfg.verbose = true
		case 'm':
			// -m requires a value: rest of this arg or next arg.
			rest := flags[i+1:]
			if len(rest) > 0 {
				cfg.mode = rest
				return 0, nil
			}
			if idx+1 >= len(args) {
				return 0, fmt.Errorf("option requires an argument -- 'm'")
			}
			cfg.mode = args[idx+1]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
