// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/pwd: print working directory.
// Implements srd051-pwd R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "pwd"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: pwd [OPTION]...
Print the full filename of the current working directory.

  -L, --logical   use PWD from environment, even if it contains symlinks
  -P, --physical  avoid all symlinks
      --help     display this help and exit
      --version  output version information and exit
`

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// mode represents the pwd operating mode.
type mode int

const (
	modePhysical mode = iota
	modeLogical
)

func main() {
	sys.InstallSIGPIPEHandler()

	m, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	dir, err := getwd(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}

	fmt.Println(dir)
}

// getwd returns the working directory according to the given mode.
// R1.2: logical mode uses PWD if valid; falls back to physical.
// R1.3: physical mode resolves all symlinks via os.Getwd.
func getwd(m mode) (string, error) {
	if m == modeLogical {
		if dir, ok := validLogicalPWD(); ok {
			return dir, nil
		}
	}
	return os.Getwd()
}

// validLogicalPWD checks whether PWD is set, absolute, contains no
// . or .. components, and refers to the actual working directory.
// R1.2: PWD must be absolute and match the physical directory.
func validLogicalPWD() (string, bool) {
	pwd := os.Getenv("PWD")
	if pwd == "" || !filepath.IsAbs(pwd) {
		return "", false
	}
	if containsDotComponents(pwd) {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(pwd)
	if err != nil {
		return "", false
	}
	physical, err := os.Getwd()
	if err != nil {
		return "", false
	}
	if resolved != physical {
		return "", false
	}
	return pwd, true
}

// containsDotComponents reports whether path has . or .. path elements.
func containsDotComponents(path string) bool {
	for part := range strings.SplitSeq(path, string(filepath.Separator)) {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}

// parseArgs processes command-line arguments and returns the selected mode.
// R1.4: last flag wins when both -L and -P are given.
// R2.1: extra operands produce an error.
// R2.2: unknown flags produce an error.
func parseArgs(args []string) (mode, error) {
	m := modePhysical
	for _, arg := range args {
		switch arg {
		case "--help":
			fmt.Print(helpText)
			os.Exit(0)
		case "--version":
			fmt.Println(versionText)
			os.Exit(0)
		case "-L", "--logical":
			m = modeLogical
		case "-P", "--physical":
			m = modePhysical
		default:
			if strings.HasPrefix(arg, "-") && arg != "-" {
				return 0, fmt.Errorf("unrecognized option '%s'", arg)
			}
			return 0, fmt.Errorf("extra operand '%s'", arg)
		}
	}
	return m, nil
}
