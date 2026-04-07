// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/dirname: strip last component from file paths.
// Implements srd016-dirname R1.1-R1.5, R2.1, R2.2.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "dirname"

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.1: -z/--zero terminates output with NUL instead of newline.
	zeroFlag := flag.Bool("z", false, "end each output line with NUL, not newline")
	flag.BoolVar(zeroFlag, "zero", false, "end each output line with NUL, not newline")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	terminator := "\n"
	if *zeroFlag {
		terminator = "\x00"
	}

	// R1.5: process multiple NAME arguments in order.
	// R2.2: results printed in argument order.
	for _, arg := range args {
		fmt.Print(dirname(arg) + terminator)
	}
}

// usage prints the usage message to stderr.
func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION] NAME...\n", progName)
	fmt.Fprintf(os.Stderr, "Output each NAME with its last non-slash component"+
		" and trailing slashes removed;\n")
	fmt.Fprintf(os.Stderr, "if NAME contains no /'s, output '.'"+
		" (meaning the current directory).\n\n")
	flag.PrintDefaults()
}

// dirname extracts the directory component from a pathname.
// R1.1: strip trailing slashes, then remove everything after the last '/'.
// R1.2: if no '/' remains after trailing-slash removal, return ".".
// R1.3: if the name is entirely slashes, return "/".
// R1.4: strip trailing slashes from the result; if empty, return "/".
func dirname(name string) string {
	// R1.2: empty string has no slash, return ".".
	if name == "" {
		return "."
	}

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
