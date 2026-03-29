// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pwd implements GNU pwd: print the current working directory.
//
// Implements prd051-pwd R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "pwd"

// pwdMode represents the logical vs physical path resolution mode.
type pwdMode int

const (
	// R1.1, R1.3: physical mode is the default — resolve symlinks.
	modePhysical pwdMode = iota
	// R1.2: logical mode — use PWD environment variable.
	modeLogical
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the working directory. Returns exit code.
func run(args []string, stdout, stderr *os.File) int {
	m, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		return 1
	}
	return printWorkingDir(m, stdout, stderr)
}

// parseArgs extracts the mode from command-line arguments.
// R1.4: when both -L and -P are given, the last one wins.
// R2.1: positional operands are rejected.
// R2.2: unknown flags produce an error.
func parseArgs(args []string) (pwdMode, error) {
	m := modePhysical // R1.1: default is physical.
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return m, fmt.Errorf("extra operand '%s'", arg)
		}
		parsed, err := parseFlag(arg, m)
		if err != nil {
			return m, err
		}
		m = parsed
	}
	return m, nil
}

// parseFlag parses a single flag argument and returns the updated mode.
func parseFlag(arg string, current pwdMode) (pwdMode, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, current)
	}
	return parseShortFlags(arg, current)
}

// parseLongFlag handles --logical and --physical.
func parseLongFlag(arg string, current pwdMode) (pwdMode, error) {
	switch arg {
	case "--logical":
		return modeLogical, nil
	case "--physical":
		return modePhysical, nil
	default:
		return current, fmt.Errorf("unrecognized option '%s'", arg)
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
// R1.2: returns "" if PWD is unset, not absolute, contains dot
// components, or does not refer to the same directory as os.Getwd().
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

// sameDirectory reports whether path refers to the same directory as os.Getwd().
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
