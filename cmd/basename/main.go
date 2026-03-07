// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU basename: strip directory and suffix from filenames.
// Implements prd015-basename R1-R4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R4: Handle --help and --version before flag parsing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Println("Usage: basename NAME [SUFFIX]")
			fmt.Println("  or:  basename OPTION... NAME...")
			fmt.Println("Print NAME with any leading directory components removed.")
			fmt.Println("If specified, also remove a trailing SUFFIX.")
			os.Exit(0)
		case "--version":
			fmt.Println("basename (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	// Parse flags manually (D4).
	multiple := false
	zeroTerminated := false
	suffix := ""
	var names []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-a" || arg == "--multiple" {
			multiple = true
			i++
			continue
		}
		if arg == "-z" || arg == "--zero" {
			zeroTerminated = true
			i++
			continue
		}
		if arg == "-s" || arg == "--suffix" {
			// R2.2: -s implies -a.
			multiple = true
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "basename: option requires an argument -- 's'\n")
				os.Exit(1)
			}
			suffix = args[i]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--suffix=") {
			// R2.2: --suffix= implies -a.
			multiple = true
			suffix = arg[len("--suffix="):]
			i++
			continue
		}
		if strings.HasPrefix(arg, "-s") && len(arg) > 2 {
			// -sSUFFIX form: suffix immediately follows -s.
			multiple = true
			suffix = arg[2:]
			i++
			continue
		}
		// Combined short flags like -az, -za.
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			allFlags := true
			for _, ch := range arg[1:] {
				if ch != 'a' && ch != 'z' {
					allFlags = false
					break
				}
			}
			if allFlags {
				for _, ch := range arg[1:] {
					switch ch {
					case 'a':
						multiple = true
					case 'z':
						zeroTerminated = true
					}
				}
				i++
				continue
			}
		}
		// Not a flag; stop parsing.
		break
	}

	names = args[i:]

	// R3.3, R3.4: No arguments is an error.
	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "basename: missing operand\n")
		os.Exit(1)
	}

	// R3.3: In single-argument mode, more than two positional args is an error.
	if !multiple && len(names) > 2 {
		fmt.Fprintf(os.Stderr, "basename: extra operand '%s'\n", names[2])
		os.Exit(1)
	}

	terminator := "\n"
	if zeroTerminated {
		terminator = "\x00"
	}

	if !multiple {
		// Single-argument mode: basename NAME [SUFFIX]
		name := names[0]
		suf := ""
		if len(names) == 2 {
			suf = names[1]
		}
		result := stripBasename(name, suf)
		fmt.Print(result + terminator)
	} else {
		// Multi-argument mode: process each NAME with the -s suffix.
		for _, name := range names {
			result := stripBasename(name, suffix)
			fmt.Print(result + terminator)
		}
	}
}

// stripBasename strips directory components and optionally removes a suffix.
// R1.1: Strip longest prefix ending in '/'.
// R1.2: Remove SUFFIX if it matches and is not the entire basename.
// R1.3: Strip trailing slashes first.
// R1.4: All-slash input yields '/'.
// R1.5: Empty input yields empty string.
func stripBasename(name, suffix string) string {
	if name == "" {
		return ""
	}

	// R1.3: Strip trailing slashes.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}

	// R1.4: If name is just "/", return "/".
	if name == "/" {
		return "/"
	}

	// R1.1: Strip directory prefix (everything up to and including the last '/').
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// R1.2: Remove suffix if provided and not equal to the entire basename.
	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}
