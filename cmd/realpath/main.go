// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd049-realpath R1.1-R1.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: realpath [OPTION]... FILE...
Print the resolved absolute file name;
all but the last component must exist

  -e, --canonicalize-existing  all components of the path must exist
  -m, --canonicalize-missing   no path components need exist or be a directory
      --help                   display this help and exit
      --version                output version information and exit
`

const versionText = `realpath (go-unix-utils) dev
`

type resolveMode int

const (
	modeDefault resolveMode = iota
	modeCanonE
	modeCanonM
)

func main() {
	sys.InstallSIGPIPEHandler()

	m, operands := parseArgs(os.Args[1:])
	if len(operands) == 0 {
		fmt.Fprintln(os.Stderr, "realpath: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, op := range operands {
		result, err := resolve(m, op)
		if err != nil {
			fmt.Fprintf(os.Stderr, "realpath: %s: %s\n", op, sysError(err))
			exitCode = 1
			continue
		}
		fmt.Fprintln(os.Stdout, result)
	}
	os.Exit(exitCode)
}

func parseArgs(args []string) (resolveMode, []string) {
	m := modeDefault
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
		case arg == "--canonicalize-existing":
			m = modeCanonE
		case arg == "--canonicalize-missing":
			m = modeCanonM
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "realpath: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			m = parseShortFlags(arg[1:], m)
		default:
			operands = append(operands, arg)
		}
	}
	return m, operands
}

func parseShortFlags(flags string, m resolveMode) resolveMode {
	for _, ch := range flags {
		switch ch {
		case 'e':
			m = modeCanonE
		case 'm':
			m = modeCanonM
		default:
			fmt.Fprintf(os.Stderr, "realpath: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
			os.Exit(1)
		}
	}
	return m
}

func resolve(m resolveMode, path string) (string, error) {
	switch m {
	case modeCanonE:
		return canonicalizeE(path)
	case modeCanonM:
		return canonicalizeM(path)
	default:
		return canonicalizeDefault(path)
	}
}

func canonicalizeDefault(path string) (string, error) {
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

func sysError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		msg := pe.Err.Error()
		if len(msg) > 0 {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
		return msg
	}
	return err.Error()
}
