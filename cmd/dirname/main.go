// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU dirname: strip last component from file names.
// Implements prd016-dirname R1-R4.
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

	// R3: Handle --help and --version before flag parsing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Println("Usage: dirname [OPTION] NAME...")
			fmt.Println("Output each NAME with its last non-slash component and trailing slashes")
			fmt.Println("removed; if NAME contains no /'s, output '.' (meaning the current directory).")
			os.Exit(0)
		case "--version":
			fmt.Println("dirname (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	// Parse flags manually (D2: match basename pattern).
	zeroTerminated := false

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-z" || arg == "--zero" {
			zeroTerminated = true
			i++
			continue
		}
		// Combined short flags like -z (only z is valid for dirname).
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			allFlags := true
			for _, ch := range arg[1:] {
				if ch != 'z' {
					allFlags = false
					break
				}
			}
			if allFlags {
				zeroTerminated = true
				i++
				continue
			}
		}
		// Not a flag; stop parsing.
		break
	}

	names := args[i:]

	// R3.2: No arguments is an error.
	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "dirname: missing operand\n")
		os.Exit(1)
	}

	terminator := "\n"
	if zeroTerminated {
		terminator = "\x00"
	}

	// R1.5: Process each argument.
	for _, name := range names {
		result := stripDirname(name)
		fmt.Print(result + terminator)
	}
}

// stripDirname returns the directory component of name, matching GNU dirname behavior.
// R1.1: Strip trailing slashes, then remove the last component.
// R1.2: If no '/' remains, return ".".
// R1.3: All-slash input yields "/".
// R1.4: Strip trailing slashes from result.
func stripDirname(name string) string {
	if name == "" {
		return "."
	}

	// R1.3: If name is all slashes, return "/".
	allSlashes := true
	for _, ch := range name {
		if ch != '/' {
			allSlashes = false
			break
		}
	}
	if allSlashes {
		return "/"
	}

	// R1.1: Strip trailing slashes.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}

	// R1.2: If no slash remains, return ".".
	idx := strings.LastIndex(name, "/")
	if idx < 0 {
		return "."
	}

	// Remove the last component.
	dir := name[:idx]

	// R1.4: Strip trailing slashes from result, but not if it would become empty.
	for len(dir) > 1 && dir[len(dir)-1] == '/' {
		dir = dir[:len(dir)-1]
	}

	if dir == "" {
		return "/"
	}

	return dir
}
