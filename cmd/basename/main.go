// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd015-basename R1.1–R1.4: strip directory and suffix from
// filenames, matching GNU coreutils basename behavior.
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
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "basename: missing operand\nTry 'basename --help' for more information.\n")
		os.Exit(1)
	}
	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "basename: extra operand '%s'\nTry 'basename --help' for more information.\n", args[2])
		os.Exit(1)
	}

	name := args[0]
	suffix := ""
	if len(args) == 2 {
		suffix = args[1]
	}

	fmt.Println(basename(name, suffix))
}

// basename strips the directory prefix and optional suffix from name,
// matching GNU coreutils behavior.
func basename(name, suffix string) string {
	// R1.5: empty string produces empty output.
	if name == "" {
		return ""
	}

	// R1.4: a name consisting entirely of slashes produces "/".
	if allSlashes(name) {
		return "/"
	}

	// R1.3: strip trailing slashes before extracting the base component.
	name = strings.TrimRight(name, "/")

	// R1.1: strip the longest prefix ending in '/'.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	// R1.2: remove literal suffix if provided and result would not be empty.
	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}

// allSlashes reports whether s is non-empty and consists entirely of '/'.
func allSlashes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}
