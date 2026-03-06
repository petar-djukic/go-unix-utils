// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/dirname: strip last component from file paths.
// Implements prd016-dirname R1, R2, R3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	binName = "dirname"
	version = "0.1.0"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var zero bool

	// Parse flags manually, matching cmd/yes and cmd/true convention.
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		switch arg {
		case "--help":
			printHelp()
			os.Exit(0)
		case "--version":
			if _, err := fmt.Printf("%s (go-unix-utils) %s\n", binName, version); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--zero":
			zero = true
			continue
		case "-z":
			zero = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: only -z is valid.
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'z':
					zero = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\nTry '%s --help' for more information.\n", binName, arg[j], binName)
					os.Exit(1)
				}
			}
			continue
		}
		positional = append(positional, arg)
	}

	// R3.2: no arguments is an error.
	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\nTry '%s --help' for more information.\n", binName, binName)
		os.Exit(1)
	}

	terminator := "\n"
	if zero {
		terminator = "\x00"
	}

	// R1.5: process multiple NAME arguments, one result per argument.
	for _, name := range positional {
		result := dirnameStr(name)
		if _, err := fmt.Print(result + terminator); err != nil {
			// R3.3: exit 1 on write error.
			os.Exit(1)
		}
	}
}

// dirnameStr strips the last component from name, matching GNU dirname behavior.
// R1.1: strip trailing slashes, then remove last component.
// R1.2: no '/' after stripping → return ".".
// R1.3: all-slash input → return "/".
// R1.4: strip trailing slashes from result; if empty, return "/".
func dirnameStr(name string) string {
	// Strip trailing slashes (unless the entire string is slashes).
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}
	// All slashes.
	if name == "/" {
		return "/"
	}
	// Find last slash.
	i := strings.LastIndex(name, "/")
	if i < 0 {
		// R1.2: no directory component.
		return "."
	}
	// Remove everything after last slash.
	name = name[:i]
	// Strip trailing slashes from result.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}
	// R1.4: if result is empty, return "/".
	if name == "" {
		return "/"
	}
	return name
}

func printHelp() {
	_, err := fmt.Printf("Usage: %s [OPTION] NAME...\nOutput each NAME with its last non-slash component and trailing slashes\nremoved; if NAME contains no /'s, output '.' (meaning the current directory).\n\n  -z, --zero           end each output line with NUL, not newline\n      --help        display this help and exit\n      --version     output version information and exit\n", binName)
	if err != nil {
		os.Exit(1)
	}
}
