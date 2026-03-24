// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd035-rmdir: Remove Empty Directories.
// Covers R1.1-R1.4 (basic directory removal, error handling),
// R2.1-R2.3 (parent removal with -p/--parents, independent arg processing),
// R3.1-R3.4 (--ignore-fail-on-non-empty, verbose, exit codes, --version/--help).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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
		fmt.Fprintln(os.Stderr, "rmdir: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'rmdir --help' for more information.")
		os.Exit(1)
	}

	os.Exit(run(cfg, dirs))
}

// config holds parsed flag state.
type config struct {
	parents            bool
	ignoreFailNonEmpty bool
	verbose            bool
}

// run removes directories and returns the exit code.
// R1.2: processes each directory independently, left to right.
// R3.4: exits 0 on full success, 1 if any removal fails.
func run(cfg config, dirs []string) int {
	exitCode := 0
	for _, dir := range dirs {
		if err := removeDir(cfg, dir); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// removeDir removes a single directory according to config.
func removeDir(cfg config, dir string) error {
	if cfg.parents {
		return removeWithParents(cfg, dir)
	}
	return removeSingle(cfg, dir)
}

// removeSingle removes a single directory without parent traversal.
// R1.1: removes one empty directory.
// R1.3: fails if directory is not empty.
// R1.4: fails if target does not exist.
func removeSingle(cfg config, dir string) error {
	err := syscall.Rmdir(dir)
	if err == nil {
		// R3.3: verbose diagnostic on successful removal.
		if cfg.verbose {
			printVerbose(dir)
		}
		return nil
	}
	return handleRemoveError(cfg, dir, err)
}

// removeWithParents removes a directory and its empty ancestors.
// R2.1: removes target, then each ancestor component.
// R2.2: stops when a parent is not empty.
func removeWithParents(cfg config, dir string) error {
	clean := filepath.Clean(dir)
	if err := removeSingle(cfg, clean); err != nil {
		return err
	}
	return removeAncestors(cfg, clean)
}

// removeAncestors walks up the directory path removing ancestors.
func removeAncestors(cfg config, dir string) error {
	parent := filepath.Dir(dir)
	for parent != "." && parent != "/" {
		err := syscall.Rmdir(parent)
		if err == nil {
			// R3.3: verbose diagnostic for each ancestor removed.
			if cfg.verbose {
				printVerbose(parent)
			}
			parent = filepath.Dir(parent)
			continue
		}
		return handleParentError(cfg, parent, err)
	}
	return nil
}

// handleRemoveError reports or suppresses a removal error.
// R3.1: --ignore-fail-on-non-empty suppresses ENOTEMPTY/EEXIST.
// R3.2: other errors are never suppressed.
func handleRemoveError(cfg config, dir string, err error) error {
	if cfg.ignoreFailNonEmpty && isNonEmptyError(err) {
		return nil
	}
	printRemoveError(dir, err)
	return err
}

// handleParentError reports or suppresses a parent removal error.
// GNU uses "failed to remove directory" for -p ancestor errors.
func handleParentError(cfg config, dir string, err error) error {
	if cfg.ignoreFailNonEmpty && isNonEmptyError(err) {
		return nil
	}
	reason := capitalizeFirst(err.Error())
	fmt.Fprintf(os.Stderr, "rmdir: failed to remove directory '%s': %s\n",
		dir, reason)
	return err
}

// isNonEmptyError returns true if the error indicates a non-empty
// directory (ENOTEMPTY or EEXIST on some systems).
func isNonEmptyError(err error) bool {
	return err == syscall.ENOTEMPTY || err == syscall.EEXIST
}

// printRemoveError formats a removal error in GNU style.
func printRemoveError(dir string, err error) {
	reason := capitalizeFirst(err.Error())
	fmt.Fprintf(os.Stderr, "rmdir: failed to remove '%s': %s\n",
		dir, reason)
}

// printVerbose prints a verbose diagnostic for a removed directory.
// R3.3: GNU rmdir prints verbose output to stdout.
func printVerbose(dir string) {
	fmt.Fprintf(os.Stdout, "rmdir: removing directory, '%s'\n", dir)
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
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
			cfg.parents = true
		case arg == "-v" || arg == "--verbose":
			cfg.verbose = true
		case arg == "--ignore-fail-on-non-empty":
			cfg.ignoreFailNonEmpty = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			exit = parseShortFlags(arg, &cfg)
			if exit >= 0 {
				return cfg, nil, exit
			}
		default:
			dirs = append(dirs, args[i:]...)
			return
		}
	}
	return
}

// parseShortFlags handles combined single-char flags like -pv.
// Returns -1 to continue, >= 0 for early exit.
func parseShortFlags(arg string, cfg *config) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'p':
			cfg.parents = true
		case 'v':
			cfg.verbose = true
		default:
			fmt.Fprintf(os.Stderr,
				"rmdir: unrecognized option '-%c'\n", arg[j])
			return 1
		}
	}
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: rmdir [OPTION]... DIRECTORY...
Remove the DIRECTORY(ies), if they are empty.

      --ignore-fail-on-non-empty
                  ignore each failure that is solely because a directory
                    is non-empty
  -p, --parents   remove DIRECTORY and its ancestors; e.g., 'rmdir -p a/b/c' is
                    similar to 'rmdir a/b/c a/b a'
  -v, --verbose   output a diagnostic for every directory processed

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
		"rmdir (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
