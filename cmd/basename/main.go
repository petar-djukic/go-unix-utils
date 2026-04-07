// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/basename: strip directory and suffix from filenames.
// Implements srd015-basename R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "basename"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 || len(args) > 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		if len(args) > 2 {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, args[2])
		}
		os.Exit(1)
	}

	name := args[0]
	suffix := ""
	if len(args) == 2 {
		suffix = args[1]
	}

	result := basename(name, suffix)
	fmt.Println(result)
}

// basename strips the directory prefix and optional suffix from name.
// R1.3: trailing slashes are removed before processing.
// R1.4: a name consisting entirely of slashes returns "/".
// R1.1: the longest prefix ending in '/' is stripped.
// R1.2: suffix is removed from the end if it matches (literal).
func basename(name, suffix string) string {
	// R1.5: empty string produces empty output.
	if name == "" {
		return ""
	}

	// R1.3: strip trailing slashes.
	trimmed := strings.TrimRight(name, "/")

	// R1.4: if name was entirely slashes, result is "/".
	if trimmed == "" {
		return "/"
	}
	name = trimmed

	// R1.1: strip longest prefix ending in '/'.
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// R1.2: remove suffix if present and not equal to the entire name.
	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}
