// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd016-dirname R1.1–R1.4: strip last component from
// file paths, outputting the directory portion.
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
	if len(args) > 0 && args[0] == "--help" {
		printUsage()
		return
	}
	if len(args) == 0 {
		printError("missing operand")
		os.Exit(1)
	}
	for _, name := range args {
		fmt.Println(dirname(name))
	}
}

// printUsage writes usage information to stdout. Implements R1.4.
func printUsage() {
	fmt.Print("Usage: dirname [OPTION] NAME...\n" +
		"Output each NAME with its last non-slash component " +
		"and trailing slashes removed;\n" +
		"if NAME contains no /'s, output '.' " +
		"(meaning the current directory).\n\n" +
		"      --help     display this help and exit\n")
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"dirname: %s\nTry 'dirname --help' for more information.\n", msg)
}

// dirname extracts the directory component from name, matching GNU
// coreutils behavior. Implements R1.1–R1.3.
func dirname(name string) string {
	if name == "" {
		return "."
	}
	if allSlashes(name) {
		return "/"
	}
	// R1.2: strip trailing slashes before extracting directory.
	name = strings.TrimRight(name, "/")
	i := strings.LastIndex(name, "/")
	if i < 0 {
		// R1.3: no slash means current directory.
		return "."
	}
	// Strip trailing slashes from the result.
	dir := strings.TrimRight(name[:i], "/")
	if dir == "" {
		return "/"
	}
	return dir
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
