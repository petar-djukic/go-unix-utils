// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/basename implements GNU basename: strip directory and suffix from filenames.
//
// Implements prd015-basename R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// helpText is the usage message printed when --help is passed (R1.3).
const helpText = `Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

Mandatory arguments to long options are mandatory for short options too.
  -a, --multiple       support multiple arguments and treat each as a NAME
  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a
  -z, --zero           end each output line with NUL, not newline
      --help        display this help and exit
      --version     output version information and exit

Examples:
  basename /usr/bin/sort          -> "sort"
  basename include/stdio.h .h     -> "stdio"
  basename -s .h include/stdio.h  -> "stdio"
  basename -a any/str1 any/str2   -> "str1" followed by "str2"
`

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and executes basename logic.
// Returns exit code: 0 on success, 1 on error.
func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "basename: missing operand")
		fmt.Fprintln(stderr, "Try 'basename --help' for more information.")
		return 1
	}

	if args[0] == "--help" {
		fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort output
		return 0
	}

	if len(args) > 2 {
		fmt.Fprintf(stderr, "basename: extra operand '%s'\n", args[2])
		fmt.Fprintln(stderr, "Try 'basename --help' for more information.")
		return 1
	}

	name := args[0]
	suffix := ""
	if len(args) == 2 {
		suffix = args[1]
	}

	result := basename(name, suffix)
	fmt.Fprintln(stdout, result) //nolint:errcheck // best-effort output
	return 0
}

// basename strips directory components and optional suffix from name.
// R1.1: strips the longest prefix ending in '/'.
// R1.3: trailing slashes are removed first.
// R1.4: if name is empty or all slashes, returns "/" for all-slashes or "" for empty.
func basename(name, suffix string) string {
	if name == "" {
		return ""
	}

	// R1.4: if name consists entirely of slashes, return "/".
	if allSlashes(name) {
		return "/"
	}

	// R1.3: strip trailing slashes.
	name = strings.TrimRight(name, "/")

	// R1.1: strip longest prefix ending in '/'.
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// R1.2: remove suffix if it matches and doesn't consume the entire name.
	if suffix != "" && suffix != name && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}

// allSlashes returns true if s consists entirely of '/' characters.
func allSlashes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}
