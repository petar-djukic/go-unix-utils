// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/rmdir: remove empty directories.
// Implements srd035 R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "rmdir"

// usageText is the --help output printed to stdout.
const usageText = `Usage: rmdir [OPTION]... DIRECTORY...
Remove the DIRECTORY(ies), if they are empty.

      --ignore-fail-on-non-empty
                  ignore each failure that is solely because a directory
                    is non-empty
  -p, --parents   remove DIRECTORY and its ancestors; e.g., 'rmdir -p a/b/c' is
                    similar to 'rmdir a/b/c a/b a'
  -v, --verbose   output a diagnostic for every directory processed
      --help      display this help and exit
      --version   output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "rmdir (go-unix-utils) 0.1.0\n"

// config holds parsed command-line options for rmdir.
type config struct {
	parents              bool // -p, --parents (stub, implemented in later task)
	ignoreFail           bool // --ignore-fail-on-non-empty (stub)
	verbose              bool // -v, --verbose (stub)
	help                 bool // --help
	version              bool // --version
	dirs                 []string
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

// run executes the rmdir logic and returns the exit code.
// R1.2: processes each directory argument independently.
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
		if err := removeDirEntry(dir, cfg.parents); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// removeDirEntry removes a directory, optionally ascending through parents.
// R1.1, R2.1: uses os.Remove which calls rmdir(2) for directories.
// R2.2: stops ascending when a parent removal fails.
// R2.3: each DIR argument is processed independently via the caller loop.
func removeDirEntry(dir string, parents bool) error {
	if err := os.Remove(dir); err != nil {
		return formatRmdirError(dir, err)
	}
	if parents {
		return removeParents(dir)
	}
	return nil
}

// removeParents ascends through parent components of path, removing each
// empty directory until a removal fails or the path is exhausted.
// R2.1: removes successive parent components after the target.
// R2.2: stops and reports error when a parent is not empty.
func removeParents(dir string) error {
	parent := filepath.Dir(dir)
	for parent != "." && parent != "/" {
		if err := os.Remove(parent); err != nil {
			return formatRmdirError(parent, err)
		}
		parent = filepath.Dir(parent)
	}
	return nil
}

// formatRmdirError wraps a remove error to match GNU rmdir output format.
// R1.4: "rmdir: failed to remove 'NAME': REASON"
func formatRmdirError(dir string, err error) error {
	return fmt.Errorf("failed to remove '%s': %s", dir, unwrapOSError(err))
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
		return parseLongFlag(cfg, arg)
	}
	return parseShortFlags(cfg, arg)
}

// parseLongFlag handles --name flags.
// R1.3: defines -p, --ignore-fail-on-non-empty, --verbose as stubs (D4).
func parseLongFlag(cfg *config, arg string) (int, error) {
	switch arg {
	case "--parents":
		cfg.parents = true
	case "--ignore-fail-on-non-empty":
		cfg.ignoreFail = true
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
// R1.3: defines -p and -v as stubs (D4).
func parseShortFlags(cfg *config, arg string) (int, error) {
	flags := arg[1:]
	for _, ch := range flags {
		switch ch {
		case 'p':
			cfg.parents = true
		case 'v':
			cfg.verbose = true
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
