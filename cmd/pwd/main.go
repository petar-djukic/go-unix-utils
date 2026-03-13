// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd051-pwd R1.1–R1.4, R2.1–R2.2, R3.1–R3.3
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "pwd"

// pwdMode controls logical vs physical path resolution.
type pwdMode int

const (
	// modePhysical (-P) resolves symlinks. This is the default (R1.1).
	modePhysical pwdMode = iota
	// modeLogical (-L) uses the PWD environment variable (R1.2).
	modeLogical
)

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	mode := modePhysical // R1.1: default is physical.
	hasOperands := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--":
			// End of flags; remaining args are operands.
			if i+1 < len(args) {
				hasOperands = true
			}
			i = len(args) // skip remaining
		case arg == "--logical":
			// R1.2: --logical long form of -L.
			mode = modeLogical
		case arg == "--physical":
			// R1.3: --physical long form of -P.
			mode = modePhysical
		case strings.HasPrefix(arg, "--"):
			// R2.2: unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Short flag cluster. R1.4: last flag wins.
			cluster := arg[1:]
			for _, ch := range cluster {
				switch ch {
				case 'L':
					// R1.2: logical mode.
					mode = modeLogical
				case 'P':
					// R1.3: physical mode.
					mode = modePhysical
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, ch)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			// R2.1: operands are noted and warned about, matching gpwd behavior.
			hasOperands = true
		}
	}

	// R2.1: warn about non-option arguments, matching gpwd behavior.
	if hasOperands {
		fmt.Fprintf(os.Stderr, "%s: ignoring non-option arguments\n", programName)
	}

	var result string
	var err error

	switch mode {
	case modeLogical:
		// R1.2: use PWD if it is absolute, has no . or .. components, and refers
		// to the same directory as the physical cwd. Fall back to physical otherwise.
		result, err = logicalPwd()
	case modePhysical:
		// R1.3: resolve symlinks. os.Getwd() may return a PWD-based path
		// containing symlinks (Go checks PWD env before syscall), so
		// EvalSymlinks is needed to match GNU pwd -P behavior.
		var cwd string
		cwd, err = os.Getwd()
		if err == nil {
			result, err = filepath.EvalSymlinks(cwd)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}

	fmt.Println(result)
}

// logicalPwd returns the logical working directory from the PWD environment variable.
// R1.2: PWD must be absolute, contain no . or .. components, and refer to the same
// directory (same device and inode) as the physical cwd. If any check fails, falls
// back to physical mode (os.Getwd).
func logicalPwd() (string, error) {
	pwd := os.Getenv("PWD") // platform context: process environment, not config
	if pwd == "" || !filepath.IsAbs(pwd) {
		return os.Getwd()
	}

	// Check for . or .. components.
	for _, component := range strings.Split(pwd, "/") {
		if component == "." || component == ".." {
			return os.Getwd()
		}
	}

	// Verify PWD refers to the same directory as the physical cwd.
	pwdInfo, err := os.Stat(pwd)
	if err != nil {
		return os.Getwd()
	}

	physicalCwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cwdInfo, err := os.Stat(physicalCwd)
	if err != nil {
		return "", err
	}

	if !os.SameFile(pwdInfo, cwdInfo) {
		return physicalCwd, nil
	}

	return pwd, nil
}

// printHelp writes usage information to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: pwd [OPTION]...
Print the full filename of the current working directory.

  -L, --logical    use PWD from environment, even if it contains symlinks
  -P, --physical   avoid all symlinks
      --help     display this help and exit
      --version  output version information and exit

NOTE: your shell may have its own version of pwd, which usually supersedes
the version described here.  Please refer to your shell's documentation
for details about the options it supports.
`)
}

// printVersion writes version information to stdout and exits 0.
func printVersion() {
	fmt.Println("pwd (go-unix-utils) 0.1")
}
