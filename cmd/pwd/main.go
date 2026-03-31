// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pwd implements GNU pwd: print the current working directory.
//
// Implements prd051-pwd R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "pwd"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// pwdMode represents the logical vs physical path resolution mode.
type pwdMode int

const (
	// R1.1, R1.3: physical mode is the default — resolve symlinks.
	modePhysical pwdMode = iota
	// R1.2: logical mode — use PWD environment variable.
	modeLogical
)

// parseResult describes the outcome of argument parsing.
type parseResult int

const (
	resultContinue parseResult = iota
	resultHelp
	resultVersion
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the working directory. Returns exit code.
func run(args []string, stdout, stderr *os.File) int {
	m, result, hasNonOpt, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		printTryHelp(stderr)
		return 1
	}
	// R3.1: --help prints usage information to stdout and exits 0.
	if result == resultHelp {
		printHelp(stdout)
		return 0
	}
	// R3.2: --version prints version information to stdout and exits 0.
	if result == resultVersion {
		printVersion(stdout)
		return 0
	}
	// R2.1: warn about non-option arguments but continue (matches gpwd).
	if hasNonOpt {
		fmt.Fprintf(stderr, "%s: ignoring non-option arguments\n", progName) //nolint:errcheck
	}
	return printWorkingDir(m, stdout, stderr)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// printHelp writes usage information to stdout (R3.1).
func printHelp(stdout *os.File) {
	fmt.Fprintf(stdout, "Usage: %s [OPTION]...\n", progName)            //nolint:errcheck
	fmt.Fprintln(stdout, "Print the full filename of the current working directory.") //nolint:errcheck
	fmt.Fprintln(stdout)                                                 //nolint:errcheck
	fmt.Fprintln(stdout, "  -L, --logical   use PWD from environment, even if it contains symlinks") //nolint:errcheck
	fmt.Fprintln(stdout, "  -P, --physical  avoid all symlinks")         //nolint:errcheck
	fmt.Fprintln(stdout, "      --help      display this help and exit") //nolint:errcheck
	fmt.Fprintln(stdout, "      --version   output version information and exit") //nolint:errcheck
}

// printVersion writes version information to stdout (R3.2).
func printVersion(stdout *os.File) {
	fmt.Fprintf(stdout, "%s (go-unix-utils) %s\n", progName, version) //nolint:errcheck
}

// parseArgs extracts the mode from command-line arguments.
// R1.4: when both -L and -P are given, the last one wins.
// R2.1: non-option arguments are warned about but ignored (matches gpwd).
// R2.2: unknown flags produce an error.
func parseArgs(args []string) (pwdMode, parseResult, bool, error) {
	m := modePhysical // R1.1: default is physical.
	hasNonOpt := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			hasNonOpt = true
			continue
		}
		parsed, result, err := parseFlag(arg, m)
		if err != nil {
			return m, resultContinue, hasNonOpt, err
		}
		if result != resultContinue {
			return parsed, result, hasNonOpt, nil
		}
		m = parsed
	}
	return m, resultContinue, hasNonOpt, nil
}

// parseFlag parses a single flag argument and returns the updated mode.
func parseFlag(arg string, current pwdMode) (pwdMode, parseResult, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, current)
	}
	m, err := parseShortFlags(arg, current)
	return m, resultContinue, err
}

// parseLongFlag handles --logical, --physical, --help, and --version.
func parseLongFlag(arg string, current pwdMode) (pwdMode, parseResult, error) {
	switch arg {
	case "--logical":
		return modeLogical, resultContinue, nil
	case "--physical":
		return modePhysical, resultContinue, nil
	case "--help":
		return current, resultHelp, nil
	case "--version":
		return current, resultVersion, nil
	default:
		return current, resultContinue, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// parseShortFlags handles short flag bundles like -L, -P, -LP.
// R1.4: last flag in the bundle wins.
func parseShortFlags(arg string, current pwdMode) (pwdMode, error) {
	m := current
	for _, ch := range arg[1:] {
		switch ch {
		case 'L':
			m = modeLogical
		case 'P':
			m = modePhysical
		default:
			return m, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return m, nil
}

// printWorkingDir prints the current working directory according to mode.
// R1.1: default physical mode prints resolved path.
// R1.2: logical mode uses PWD env var if valid, else falls back to physical.
// R1.3: physical mode resolves all symlinks.
// R2.2: exit 0 on success, exit 1 on failure with diagnostic to stderr.
func printWorkingDir(m pwdMode, stdout, stderr *os.File) int {
	var dir string
	var err error
	if m == modeLogical {
		dir = logicalDir()
	}
	if dir == "" {
		dir, err = physicalDir()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
			return 1
		}
	}
	fmt.Fprintln(stdout, dir) //nolint:errcheck
	return 0
}

// logicalDir returns the PWD environment variable if it is a valid
// absolute path to the current directory with no . or .. components.
// R1.2, R2.1: returns "" if PWD is unset, not absolute, contains dot-dot
// components, or does not refer to the same directory as os.Getwd()
// (compared by device and inode via os.SameFile).
func logicalDir() string {
	pwd := os.Getenv("PWD")
	if pwd == "" || !filepath.IsAbs(pwd) {
		return ""
	}
	if containsDotComponent(pwd) {
		return ""
	}
	if !sameDirectory(pwd) {
		return ""
	}
	return pwd
}

// containsDotComponent reports whether path contains a "." or ".." component.
func containsDotComponent(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}

// sameDirectory reports whether path refers to the same directory as os.Getwd()
// by comparing device and inode numbers via os.SameFile.
func sameDirectory(path string) bool {
	pwdInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	physical, err := os.Getwd()
	if err != nil {
		return false
	}
	physInfo, err := os.Stat(physical)
	if err != nil {
		return false
	}
	return os.SameFile(pwdInfo, physInfo)
}

// physicalDir returns the physical working directory with symlinks resolved.
// R1.3: uses os.Getwd() which returns the resolved path.
func physicalDir() (string, error) {
	return os.Getwd()
}
