// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd034-mkdir: Create Directories.
// Covers R1.1-R1.4 (basic directory creation, error handling),
// R2.1-R2.3 (parent directory creation with -p/--parents),
// R3.1-R3.4 (mode setting with -m/--mode, verbose with -v/--verbose).
package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	verbose bool
	mode    os.FileMode
	modeSet bool
}

// run creates directories and returns the exit code.
// R1.2: exits 0 on success, 1 if any creation fails.
func run(cfg config, dirs []string) int {
	exitCode := 0
	for _, dir := range dirs {
		if err := createDir(cfg, dir); err != nil {
			// R3.1-R3.3: GNU-format error to stderr.
			printDirError(dir, err)
			exitCode = 1
		}
	}
	return exitCode
}

// printDirError formats an error in GNU style:
// mkdir: cannot create directory 'NAME': Reason
func printDirError(dir string, err error) {
	reason := unwrapReason(err)
	fmt.Fprintf(os.Stderr, "mkdir: cannot create directory '%s': %s\n",
		dir, reason)
}

// unwrapReason extracts the OS-level reason from a path error.
func unwrapReason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return capitalizeFirst(pe.Err.Error())
	}
	if le, ok := err.(*os.LinkError); ok {
		return capitalizeFirst(le.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printVerbose prints verbose output for a created directory.
// R3.4: format matches GNU mkdir: "mkdir: created directory 'NAME'"
func printVerbose(dir string) {
	fmt.Fprintf(os.Stdout, "mkdir: created directory '%s'\n", dir)
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
	if err := os.Mkdir(dir, mode); err != nil {
		return err
	}
	if cfg.verbose {
		printVerbose(dir)
	}
	return nil
}

// createWithParents creates a directory and all intermediate parents.
// R2.1: creates intermediate directories as needed.
// R2.2-R2.3: does not error if target or intermediates already exist.
// D3: with -v, prints a message for each directory actually created.
func createWithParents(cfg config, dir string) error {
	if cfg.verbose {
		return createParentsVerbose(cfg, dir)
	}
	return createParentsQuiet(cfg, dir)
}

// createParentsQuiet creates parent directories without verbose output.
func createParentsQuiet(cfg config, dir string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	// R3.3: apply explicit mode to final directory only.
	if cfg.modeSet {
		return os.Chmod(dir, cfg.mode)
	}
	return nil
}

// createParentsVerbose creates parent dirs one by one, printing each.
func createParentsVerbose(cfg config, dir string) error {
	components := splitPath(dir)
	for _, component := range components {
		if err := mkdirIfNotExist(component); err != nil {
			return err
		}
	}
	// R3.3: apply explicit mode to final directory only.
	if cfg.modeSet {
		return os.Chmod(dir, cfg.mode)
	}
	return nil
}

// mkdirIfNotExist creates a directory if it doesn't exist and prints
// verbose output. Returns nil if the directory already exists.
func mkdirIfNotExist(dir string) error {
	err := os.Mkdir(dir, 0o777)
	if err == nil {
		printVerbose(dir)
		return nil
	}
	if os.IsExist(err) {
		return nil
	}
	return err
}

// splitPath returns all path prefixes that need to be created.
// For "a/b/c" returns ["a", "a/b", "a/b/c"].
func splitPath(dir string) []string {
	clean := filepath.Clean(dir)
	parts := strings.Split(clean, string(filepath.Separator))
	result := make([]string, 0, len(parts))
	for i := range parts {
		prefix := strings.Join(parts[:i+1], string(filepath.Separator))
		if prefix == "" {
			continue
		}
		result = append(result, prefix)
	}
	return result
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
			cfg.parents = true
		case arg == "-v" || arg == "--verbose":
			// R3.4: enable verbose output.
			cfg.verbose = true
		case arg == "-m":
			// R3.1: -m MODE sets permission bits.
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr,
					"mkdir: option requires an argument -- 'm'")
				return config{}, nil, 1
			}
			mode, err := parseMode(args[i])
			if err != nil {
				// R3.4: invalid mode prints error to stderr.
				fmt.Fprintf(os.Stderr,
					"mkdir: invalid mode '%s'\n", args[i])
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
		case strings.HasPrefix(arg, "--mode="):
			val := strings.TrimPrefix(arg, "--mode=")
			mode, err := parseMode(val)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"mkdir: invalid mode '%s'\n", val)
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
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

// parseShortFlags handles combined single-char flags like -pv, -pm.
// Returns -1 to continue, >= 0 for early exit.
func parseShortFlags(arg string, cfg *config) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'p':
			cfg.parents = true
		case 'v':
			cfg.verbose = true
		case 'm':
			rest := arg[j+1:]
			if rest != "" {
				return parseModeFromFlag(rest, cfg)
			}
			fmt.Fprintln(os.Stderr,
				"mkdir: option requires an argument -- 'm'")
			return 1
		default:
			fmt.Fprintf(os.Stderr,
				"mkdir: unrecognized option '-%c'\n", arg[j])
			return 1
		}
	}
	return -1
}

// parseModeFromFlag parses a mode value from a combined flag string.
func parseModeFromFlag(val string, cfg *config) int {
	mode, err := parseMode(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: invalid mode '%s'\n", val)
		return 1
	}
	cfg.mode = mode
	cfg.modeSet = true
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
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask
  -p, --parents     no error if existing, make parent directories as needed
  -v, --verbose     print a message for each created directory

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
		"mkdir (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
