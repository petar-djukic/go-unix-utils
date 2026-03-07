// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the dirname utility for stripping the last component
// from file paths.
//
// Implements prd016-dirname: core path stripping (R1), output formatting (R2),
// exit codes and error handling (R3).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "dirname (go-unix-utils) 1.0"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		zero  bool
		names []string
	)

	// Parse flags.
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--help" {
			fmt.Fprintln(os.Stdout, "Usage: dirname [OPTION] NAME...\nOutput each NAME with its last non-slash component and trailing slashes\nremoved; if NAME contains no /'s, output '.' (meaning the current directory).\n\n  -z, --zero     end each output line with NUL, not newline\n      --help     display this help and exit\n      --version  output version information and exit")
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintln(os.Stdout, version)
			os.Exit(0)
		}
		if arg == "--zero" {
			zero = true
			i++
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			// Short flags.
			for _, ch := range arg[1:] {
				switch ch {
				case 'z':
					zero = true
				default:
					fmt.Fprintf(os.Stderr, "dirname: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			i++
			continue
		}
		break
	}

	names = args[i:]

	// R3.2: No arguments is an error.
	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "dirname: missing operand\n")
		os.Exit(1)
	}

	terminator := byte('\n')
	if zero {
		terminator = 0
	}

	// R1.5: Process each argument.
	for _, name := range names {
		result := dirname(name)
		fmt.Fprint(os.Stdout, result)
		os.Stdout.Write([]byte{terminator})
	}
}

// dirname strips the last component from a path, matching GNU dirname behavior.
func dirname(name string) string {
	// R1.1: Strip trailing slashes.
	end := len(name) - 1
	for end > 0 && name[end] == '/' {
		end--
	}

	// R1.3: All slashes -> "/"
	if end == 0 && name[0] == '/' {
		return "/"
	}

	name = name[:end+1]

	// Find the last slash.
	lastSlash := -1
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			lastSlash = i
			break
		}
	}

	// R1.2: No slash means current directory.
	if lastSlash < 0 {
		return "."
	}

	// Remove everything after the last slash.
	result := name[:lastSlash]

	// R1.4: Strip trailing slashes from result.
	for len(result) > 1 && result[len(result)-1] == '/' {
		result = result[:len(result)-1]
	}

	// If result is empty, it was a root-relative path like "/foo".
	if result == "" {
		return "/"
	}

	return result
}
