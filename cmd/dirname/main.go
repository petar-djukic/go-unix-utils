// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/dirname: strip last component from file paths.
// Implements srd016-dirname R1.1-R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "dirname"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	for _, arg := range args {
		fmt.Println(dirname(arg))
	}
}

// dirname extracts the directory component from a pathname.
// R1.1: strip trailing slashes, then remove everything after the last '/'.
// R1.2: if no '/' remains after trailing-slash removal, return ".".
// R1.3: if the name is entirely slashes, return "/".
// R1.4: strip trailing slashes from the result; if empty, return "/".
func dirname(name string) string {
	// R1.3: name consisting entirely of slashes returns "/".
	trimmed := strings.TrimRight(name, "/")
	if trimmed == "" {
		return "/"
	}

	// R1.1: find the last slash to split directory from base.
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		// R1.2: no slash means current directory.
		return "."
	}

	// R1.4: strip trailing slashes from the directory portion.
	dir := strings.TrimRight(trimmed[:idx], "/")
	if dir == "" {
		return "/"
	}
	return dir
}
