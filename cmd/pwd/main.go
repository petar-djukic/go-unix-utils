// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd051-pwd R1.1-R1.4, R2.1-R2.2, R3.3.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: pwd [OPTION]...
Print the full filename of the current working directory.

  -L, --logical   use PWD from environment, even if it contains symlinks
  -P, --physical  avoid all symlinks
      --help     display this help and exit
      --version  output version information and exit

If no option is specified, -P is assumed.
`

const versionText = `pwd (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	mode := parseArgs(os.Args[1:])

	var dir string
	var err error
	if mode == "logical" {
		dir, err = logicalDir()
	} else {
		dir, err = physicalDir()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintln(os.Stdout, dir); err != nil {
		os.Exit(1)
	}
}

func parseArgs(args []string) string {
	mode := "physical"
	hasOperands := false
	for i := range len(args) {
		arg := args[i]
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--logical":
			mode = "logical"
		case arg == "--physical":
			mode = "physical"
		case arg == "--":
			if i+1 < len(args) {
				hasOperands = true
			}
			if hasOperands {
				fmt.Fprintln(os.Stderr, "pwd: ignoring non-option arguments")
			}
			return mode
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "pwd: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'pwd --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			mode = parseShortFlags(arg[1:], mode)
		default:
			hasOperands = true
		}
	}
	if hasOperands {
		fmt.Fprintln(os.Stderr, "pwd: ignoring non-option arguments")
	}
	return mode
}

func parseShortFlags(flags string, mode string) string {
	for _, ch := range flags {
		switch ch {
		case 'L':
			mode = "logical"
		case 'P':
			mode = "physical"
		default:
			fmt.Fprintf(os.Stderr, "pwd: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'pwd --help' for more information.")
			os.Exit(1)
		}
	}
	return mode
}

func physicalDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(cwd)
}

func logicalDir() (string, error) {
	pwd := os.Getenv("PWD")
	if pwd == "" || !isValidLogicalPWD(pwd) {
		return physicalDir()
	}
	return pwd, nil
}

func isValidLogicalPWD(pwd string) bool {
	if !filepath.IsAbs(pwd) {
		return false
	}
	if containsDotComponents(pwd) {
		return false
	}
	return namesSameDir(pwd)
}

func containsDotComponents(path string) bool {
	for component := range strings.SplitSeq(path, "/") {
		if component == "." || component == ".." {
			return true
		}
	}
	return false
}

func namesSameDir(pwd string) bool {
	pwdInfo, err := os.Stat(pwd)
	if err != nil {
		return false
	}
	cwdInfo, err := os.Stat(".")
	if err != nil {
		return false
	}
	return os.SameFile(pwdInfo, cwdInfo)
}
