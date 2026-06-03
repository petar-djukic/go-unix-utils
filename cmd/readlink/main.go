// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd050-readlink R1.1-R1.6, R2.1-R2.2, R3.1-R3.2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: readlink [OPTION]... FILE...
Print value of a symbolic link or canonical file name

  -f, --canonicalize            canonicalize by following every symlink in
                                every component of the given name recursively;
                                all but the last component must exist
  -e, --canonicalize-existing   canonicalize by following every symlink in
                                every component of the given name recursively,
                                all components must exist
  -m, --canonicalize-missing    canonicalize by following every symlink in
                                every component of the given name recursively,
                                without requirements on components existence
  -n, --no-newline              do not output the trailing delimiter
      --help                    display this help and exit
      --version                 output version information and exit
`

const versionText = `readlink (go-unix-utils) dev
`

type resolveMode int

const (
	modeDefault resolveMode = iota
	modeCanonF
	modeCanonE
	modeCanonM
)

func main() {
	sys.InstallSIGPIPEHandler()

	m, noNewline, operands := parseArgs(os.Args[1:])
	if len(operands) == 0 {
		fmt.Fprintln(os.Stderr, "readlink: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'readlink --help' for more information.")
		os.Exit(1)
	}

	if noNewline && len(operands) > 1 {
		fmt.Fprintln(os.Stderr, "readlink: ignoring --no-newline with multiple arguments")
	}

	exitCode := 0
	for _, op := range operands {
		result, err := resolve(m, op)
		if err != nil {
			exitCode = 1
			continue
		}
		if noNewline && len(operands) == 1 {
			fmt.Fprint(os.Stdout, result)
		} else {
			fmt.Fprintln(os.Stdout, result)
		}
	}
	os.Exit(exitCode)
}

func parseArgs(args []string) (resolveMode, bool, []string) {
	m := modeDefault
	noNewline := false
	var operands []string
	pastDashDash := false

	for _, arg := range args {
		if pastDashDash {
			operands = append(operands, arg)
			continue
		}
		switch {
		case arg == "--":
			pastDashDash = true
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--canonicalize":
			m = modeCanonF
		case arg == "--canonicalize-existing":
			m = modeCanonE
		case arg == "--canonicalize-missing":
			m = modeCanonM
		case arg == "--no-newline":
			noNewline = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "readlink: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'readlink --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			m, noNewline = parseShortFlags(arg[1:], m, noNewline)
		default:
			operands = append(operands, arg)
		}
	}
	return m, noNewline, operands
}

func parseShortFlags(flags string, m resolveMode, noNewline bool) (resolveMode, bool) {
	for _, ch := range flags {
		switch ch {
		case 'f':
			m = modeCanonF
		case 'e':
			m = modeCanonE
		case 'm':
			m = modeCanonM
		case 'n':
			noNewline = true
		default:
			fmt.Fprintf(os.Stderr, "readlink: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'readlink --help' for more information.")
			os.Exit(1)
		}
	}
	return m, noNewline
}

func resolve(m resolveMode, path string) (string, error) {
	switch m {
	case modeCanonF:
		return canonicalizeF(path)
	case modeCanonE:
		return canonicalizeE(path)
	case modeCanonM:
		return canonicalizeM(path)
	default:
		return os.Readlink(path)
	}
}

func canonicalizeF(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

func canonicalizeE(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}

func canonicalizeM(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return doCanonM(absPath)
}

func doCanonM(absPath string) (string, error) {
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}
	if absPath == "/" {
		return "/", nil
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedDir, err := doCanonM(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

