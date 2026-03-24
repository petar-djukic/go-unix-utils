// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd034-mkdir: Create Directories.
// Covers R1.1-R1.4 (basic directory creation, error handling),
// R2.1-R2.2 (parent directory creation with -p/--parents),
// R3.1-R3.2 (mode setting with -m/--mode).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, dirs, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	if len(dirs) == 0 {
		fmt.Fprintln(os.Stderr, "mkdir: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'mkdir --help' for more information.")
		os.Exit(1)
	}

	os.Exit(run(cfg, dirs))
}

// config holds parsed flag state.
type config struct {
	parents bool
	mode    os.FileMode
	modeSet bool
}

// run creates directories and returns the exit code.
// R1.2: exits 0 on success, 1 if any creation fails.
// R1.1, R1.2: processes multiple DIR arguments sequentially.
func run(cfg config, dirs []string) int {
	exitCode := 0
	for _, dir := range dirs {
		if err := createDir(cfg, dir); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// createDir creates a single directory according to config.
func createDir(cfg config, dir string) error {
	if cfg.parents {
		return createWithParents(cfg, dir)
	}
	return createSingle(cfg, dir)
}

// createSingle creates a directory without parent creation.
// R1.3: errors if directory already exists.
// R1.4: errors if parent does not exist.
func createSingle(cfg config, dir string) error {
	mode := os.FileMode(0o777)
	if cfg.modeSet {
		mode = cfg.mode
	}
	return os.Mkdir(dir, mode)
}

// createWithParents creates a directory and all intermediate parents.
// R2.1: creates intermediate directories as needed.
// R2.2: does not error if target already exists.
func createWithParents(cfg config, dir string) error {
	// R3.3 (future): intermediate dirs get default perms.
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	// R3.1: apply explicit mode to final directory if -m was given.
	if cfg.modeSet {
		return os.Chmod(dir, cfg.mode)
	}
	return nil
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early termination.
func parseArgs(args []string) (cfg config, dirs []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			dirs = append(dirs, args[i+1:]...)
			return
		case arg == "--help":
			return config{}, nil, printHelp()
		case arg == "--version":
			return config{}, nil, printVersion()
		case arg == "-p" || arg == "--parents":
			// R2.1: enable parent directory creation.
			cfg.parents = true
		case arg == "-m":
			// R3.1: -m MODE sets permission bits.
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "mkdir: option requires an argument -- 'm'")
				return config{}, nil, 1
			}
			mode, err := parseMode(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "mkdir: invalid mode '%s'\n", args[i])
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
		case strings.HasPrefix(arg, "--mode="):
			// R3.1: --mode=MODE sets permission bits.
			val := strings.TrimPrefix(arg, "--mode=")
			mode, err := parseMode(val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "mkdir: invalid mode '%s'\n", val)
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			exit = parseShortFlags(arg, args, i, &cfg)
			if exit >= 0 {
				return cfg, nil, exit
			}
			// skip consumed args for -m in combined short flags
		default:
			dirs = append(dirs, args[i:]...)
			return
		}
	}
	return
}

// parseShortFlags handles combined single-char flags like -pm.
// Returns -1 to continue, >= 0 for early exit.
func parseShortFlags(arg string, _ []string, _ int, cfg *config) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'p':
			cfg.parents = true
		case 'm':
			// Rest of this arg is the mode value, or next arg.
			rest := arg[j+1:]
			if rest != "" {
				mode, err := parseMode(rest)
				if err != nil {
					fmt.Fprintf(os.Stderr, "mkdir: invalid mode '%s'\n", rest)
					return 1
				}
				cfg.mode = mode
				cfg.modeSet = true
				return -1
			}
			// mode is next arg - but we can't advance i from here
			// Fall back to unrecognized since -m without value in combo is complex
			fmt.Fprintln(os.Stderr, "mkdir: option requires an argument -- 'm'")
			return 1
		default:
			fmt.Fprintf(os.Stderr, "mkdir: unrecognized option '-%c'\n", arg[j])
			return 1
		}
	}
	return -1
}

// parseMode parses an octal permission string.
// R3.1: MODE is an octal value like "0755" or "755".
func parseMode(s string) (os.FileMode, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(val), nil
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.1: --help prints to stdout and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask
  -p, --parents     no error if existing, make parent directories as needed

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.2: --version prints to stdout and exits 0.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "mkdir (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
