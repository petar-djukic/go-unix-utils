// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uname prints system information fields.
// Implements prd044-uname R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7, R1.8.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "uname"

func main() {
	sys.InstallSIGPIPEHandler()

	var flags unameFlags
	anyFlag := false

	if err := parseFlags(os.Args[1:], &flags, &anyFlag); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	// R1.1: no flags prints kernel name (equivalent to -s).
	if !anyFlag {
		flags.s = true
	}

	fields, err := collectFields(&flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(fields, " "))
}

// unameFlags holds the state of each individual flag.
type unameFlags struct {
	s, n, r, v, m, p, i bool
}

// parseFlags processes argv for single-character flags.
func parseFlags(args []string, flags *unameFlags, anyFlag *bool) error {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return fmt.Errorf("extra operand '%s'", arg)
		}
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
		if err := parseFlagChars(arg[1:], flags, anyFlag); err != nil {
			return err
		}
	}
	return nil
}

// parseFlagChars processes each character in a flag group.
func parseFlagChars(chars string, flags *unameFlags, anyFlag *bool) error {
	for _, ch := range chars {
		switch ch {
		case 's':
			flags.s = true
		case 'n':
			flags.n = true
		case 'r':
			flags.r = true
		case 'v':
			flags.v = true
		case 'm':
			flags.m = true
		case 'p':
			flags.p = true
		case 'i':
			flags.i = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
		*anyFlag = true
	}
	return nil
}

// collectFields gathers the requested uname fields in canonical order.
func collectFields(f *unameFlags) ([]string, error) {
	var fields []string
	// R1.2: kernel name (e.g., Darwin, Linux).
	if f.s {
		v, err := syscall.Sysctl("kern.ostype")
		if err != nil {
			return nil, fmt.Errorf("reading kernel name: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.3: network node hostname.
	if f.n {
		v, err := syscall.Sysctl("kern.hostname")
		if err != nil {
			return nil, fmt.Errorf("reading hostname: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.4: kernel release string.
	if f.r {
		v, err := syscall.Sysctl("kern.osrelease")
		if err != nil {
			return nil, fmt.Errorf("reading kernel release: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.5: kernel version string.
	if f.v {
		v, err := syscall.Sysctl("kern.version")
		if err != nil {
			return nil, fmt.Errorf("reading kernel version: %w", err)
		}
		fields = append(fields, sanitizeVersion(v))
	}
	// R1.6: machine hardware name.
	if f.m {
		v, err := syscall.Sysctl("hw.machine")
		if err != nil {
			return nil, fmt.Errorf("reading machine: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.7: processor type ("unknown" on most platforms).
	if f.p {
		fields = append(fields, "unknown")
	}
	// R1.8: hardware platform ("unknown" on most platforms).
	if f.i {
		fields = append(fields, "unknown")
	}
	return fields, nil
}

// sanitizeVersion removes trailing newlines and replaces internal
// newlines with spaces, matching GNU uname -v behavior on Darwin
// where kern.version may contain embedded newlines.
func sanitizeVersion(v string) string {
	v = strings.TrimRight(v, "\n")
	return strings.ReplaceAll(v, "\n", " ")
}
