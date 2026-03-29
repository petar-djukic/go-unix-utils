// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uname prints system information fields.
// Implements prd044-uname R1.1, R1.2, R1.3, R1.4.
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

	flagS, flagN, flagR := false, false, false
	anyFlag := false

	if err := parseFlags(os.Args[1:], &flagS, &flagN, &flagR, &anyFlag); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	// R1.1: no flags prints kernel name (equivalent to -s).
	if !anyFlag {
		flagS = true
	}

	fields, err := collectFields(flagS, flagN, flagR)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(fields, " "))
}

// parseFlags processes argv for single-character flags.
func parseFlags(args []string, flagS, flagN, flagR *bool, anyFlag *bool) error {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return fmt.Errorf("extra operand '%s'", arg)
		}
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
		for _, ch := range arg[1:] {
			switch ch {
			case 's':
				*flagS = true
			case 'n':
				*flagN = true
			case 'r':
				*flagR = true
			default:
				return fmt.Errorf("invalid option -- '%c'", ch)
			}
			*anyFlag = true
		}
	}
	return nil
}

// collectFields gathers the requested uname fields in canonical order.
func collectFields(flagS, flagN, flagR bool) ([]string, error) {
	var fields []string
	// R1.2: kernel name (e.g., Darwin, Linux).
	if flagS {
		v, err := syscall.Sysctl("kern.ostype")
		if err != nil {
			return nil, fmt.Errorf("reading kernel name: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.3: network node hostname.
	if flagN {
		v, err := syscall.Sysctl("kern.hostname")
		if err != nil {
			return nil, fmt.Errorf("reading hostname: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.4: kernel release string.
	if flagR {
		v, err := syscall.Sysctl("kern.osrelease")
		if err != nil {
			return nil, fmt.Errorf("reading kernel release: %w", err)
		}
		fields = append(fields, v)
	}
	return fields, nil
}
