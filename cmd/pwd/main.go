// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd051-pwd R1.1 (print working directory),
// R1.2 (-L logical mode via PWD env var),
// R1.3 (-P physical mode with symlinks resolved),
// R1.4 (last of -L/-P wins),
// R2.1 (extra operands produce error exit 1),
// R2.2 (unknown flags produce error exit 1).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "pwd"

// mode represents the -L/-P selection.
type mode int

const (
	modePhysical mode = iota
	modeLogical
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run processes arguments and prints the working directory.
// Returns the exit code.
func run(args []string) int {
	m, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 1
	}
	dir, err := getWorkDir(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	fmt.Println(dir)
	return 0
}

// getWorkDir returns the working directory path based on the
// selected mode. R1.2: logical uses PWD if valid. R1.3: physical
// resolves symlinks.
func getWorkDir(m mode) (string, error) {
	if m == modeLogical {
		if dir, ok := logicalDir(); ok {
			return dir, nil
		}
	}
	return physicalDir()
}

// logicalDir returns the PWD env var if it is set, absolute,
// contains no . or .. components, and refers to the same directory
// as the physical cwd. R1.2.
func logicalDir() (string, bool) {
	pwd := os.Getenv("PWD")
	if pwd == "" || !filepath.IsAbs(pwd) {
		return "", false
	}
	if containsDotComponent(pwd) {
		return "", false
	}
	return pwd, validateSameDir(pwd)
}

// containsDotComponent checks if the path contains . or ..
// as path components. R1.2.
func containsDotComponent(path string) bool {
	for _, comp := range strings.Split(path, "/") {
		if comp == "." || comp == ".." {
			return true
		}
	}
	return false
}

// validateSameDir checks that pwd refers to the same directory
// as the physical cwd by comparing os.FileInfo via os.SameFile.
func validateSameDir(pwd string) bool {
	pwdInfo, err := os.Stat(pwd)
	if err != nil {
		return false
	}
	physPath, err := physicalDir()
	if err != nil {
		return false
	}
	physInfo, err := os.Stat(physPath)
	if err != nil {
		return false
	}
	return os.SameFile(pwdInfo, physInfo)
}

// physicalDir returns the physical working directory with all
// symlinks resolved. R1.3.
func physicalDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(dir)
}

// parseArgs validates arguments and returns the selected mode.
// R1.4: last of -L/-P wins. R2.1: extra operands error.
// R2.2: unknown flags error. Default is physical (R1.1).
func parseArgs(args []string) (mode, error) {
	m := modePhysical
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				return m, fmt.Errorf("extra operand '%s'", args[i+1])
			}
			break
		}
		var err error
		m, err = handleArg(arg, m)
		if err != nil {
			return m, err
		}
	}
	return m, nil
}

// handleArg processes a single argument, returning the updated
// mode or an error for unrecognized input.
func handleArg(arg string, m mode) (mode, error) {
	switch arg {
	case "--help":
		fmt.Print(helpText())
		os.Exit(0)
	case "--version":
		fmt.Print(versionText())
		os.Exit(0)
	case "-L", "--logical":
		return modeLogical, nil
	case "-P", "--physical":
		return modePhysical, nil
	default:
		return m, classifyBadArg(arg)
	}
	return m, nil // unreachable
}

// classifyBadArg returns the appropriate error for an
// unrecognized argument: unknown flag or extra operand.
func classifyBadArg(arg string) error {
	if strings.HasPrefix(arg, "--") {
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		return fmt.Errorf("invalid option -- '%c'", arg[1])
	}
	return fmt.Errorf("extra operand '%s'", arg)
}

// helpText returns the --help output.
func helpText() string {
	return `Usage: pwd [OPTION]...
Print the full filename of the current working directory.

  -L, --logical   use PWD from environment, even if it contains symlinks
  -P, --physical  avoid all symlinks
      --help     display this help and exit
      --version  output version information and exit

NOTE: your shell may have its own version of pwd, which usually supersedes
the version described here.  Please refer to your shell's documentation
for details about the options it supports.
`
}

// versionText returns the --version output.
func versionText() string {
	return "pwd (go-unix-utils) 1.0\n"
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
